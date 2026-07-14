package egress

// file-kw: generic hash-chain tamper-evident log any-record read ingress evidence verify

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Record is anything that can produce a canonical hash of itself. The decision log predates this
// (it has its own JSONLSink with a "decision" leaf key, and existing logs depend on that shape);
// the READ log and the INGRESS log are newer evidence streams and share this generic chain, so a
// third hand-written copy of the chain logic does not exist to drift out of sync.
type Record interface {
	Hash() (string, error)
}

// genLeaf is one link of a generic chain: its sequence, the prior leaf's hash, the record, and this
// leaf's chain hash. The chain hash covers (seq, prev_hash, record_hash) via the same leafHash the
// decision log uses, so all three streams verify by the identical rule.
type genLeaf[T Record] struct {
	Seq      int64  `json:"seq"`
	PrevHash string `json:"prev_hash"`
	Record   T      `json:"record"`
	Hash     string `json:"hash"`
}

// Chain is an append-only, hash-chained JSONL log of any Record. One writer only: a second appender
// would fork the chain (same rule as the decision log's DiscardSink note).
type Chain[T Record] struct {
	mu   sync.Mutex
	w    io.Writer
	seq  int64
	head string
}

// NewChain starts a fresh chain (genesis head).
func NewChain[T Record](w io.Writer) *Chain[T] { return &Chain[T]{w: w, head: GenesisHash} }

// ResumeChain continues an existing chain from a trusted head + sequence.
func ResumeChain[T Record](w io.Writer, head string, seq int64) *Chain[T] {
	return &Chain[T]{w: w, seq: seq, head: head}
}

// Append writes one record as the next chained leaf. Fail-closed: head/seq advance ONLY after the
// write succeeds, so the in-memory chain equals what is on disk. The record is normalized through a
// JSON round-trip before hashing (the same U+FFFD fixpoint the decision log reaches), so the bytes
// hashed equal the bytes stored equal the bytes VerifyChain reads back.
func (c *Chain[T]) Append(rec T) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("egress chain: marshal record: %w", err)
	}
	var norm T
	if err := json.Unmarshal(raw, &norm); err != nil {
		return fmt.Errorf("egress chain: normalize record: %w", err)
	}
	evHash, err := norm.Hash()
	if err != nil {
		return fmt.Errorf("egress chain: record hash: %w", err)
	}
	h, err := leafHash(c.seq, c.head, evHash)
	if err != nil {
		return fmt.Errorf("egress chain: leaf hash: %w", err)
	}
	line, err := json.Marshal(genLeaf[T]{Seq: c.seq, PrevHash: c.head, Record: norm, Hash: h})
	if err != nil {
		return fmt.Errorf("egress chain: marshal leaf: %w", err)
	}
	if _, err := c.w.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("egress chain: write: %w", err)
	}
	c.head = h
	c.seq++
	return nil
}

// Head returns the current chain head (the last leaf's hash).
func (c *Chain[T]) Head() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.head
}

// Count returns how many leaves have been appended.
func (c *Chain[T]) Count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seq
}

// VerifyChain reads a generic chain back and confirms every link: sequence order, prev-hash linkage,
// the record's own hash, and the leaf hash. Strict decode (unknown fields + trailing content are
// tamper) — identical discipline to the decision log's Verify.
func VerifyChain[T Record](r io.Reader) (VerifyResult, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLeaf)

	var (
		seq   int64
		prev  = GenesisHash
		count int64
	)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		var lf genLeaf[T]
		if err := dec.Decode(&lf); err != nil {
			return VerifyResult{}, fmt.Errorf("egress chain: leaf %d: decode: %w", seq, err)
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			return VerifyResult{}, fmt.Errorf("egress chain: leaf %d: trailing content", seq)
		}
		if lf.Seq != seq {
			return VerifyResult{}, fmt.Errorf("egress chain: leaf %d: seq is %d (reordered/dropped)", seq, lf.Seq)
		}
		if lf.PrevHash != prev {
			return VerifyResult{}, fmt.Errorf("egress chain: leaf %d: prev_hash mismatch (chain broken)", seq)
		}
		evHash, err := lf.Record.Hash()
		if err != nil {
			return VerifyResult{}, fmt.Errorf("egress chain: leaf %d: record hash: %w", seq, err)
		}
		want, err := leafHash(lf.Seq, lf.PrevHash, evHash)
		if err != nil {
			return VerifyResult{}, fmt.Errorf("egress chain: leaf %d: %w", seq, err)
		}
		if lf.Hash != want {
			return VerifyResult{}, fmt.Errorf("egress chain: leaf %d: hash mismatch (tampered)", seq)
		}
		prev = lf.Hash
		seq++
		count++
	}
	if err := sc.Err(); err != nil {
		return VerifyResult{}, fmt.Errorf("egress chain: read: %w", err)
	}
	return VerifyResult{Head: prev, Count: count}, nil
}
