package egress

import "testing"

func TestSignApprovalRoundTrip(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	fp := "scale_deployment\x1fnamespace=prod\x1freplicas=3"
	tok := SignApproval(priv, fp)

	if tok == "" {
		t.Fatal("token must not be empty")
	}
	if !VerifyApproval(pub, fp, tok) {
		t.Error("a token must verify against its own fingerprint + key")
	}
	// deterministic (RFC 8032): the same action signs to the same token.
	if SignApproval(priv, fp) != tok {
		t.Error("signing must be deterministic")
	}
	// a token for one action must NOT authorize a different action.
	if VerifyApproval(pub, "scale_deployment\x1fnamespace=prod\x1freplicas=99", tok) {
		t.Error("token must not verify for a different fingerprint")
	}
	// a different key must not verify.
	otherPub, _, _ := GenerateKey()
	if VerifyApproval(otherPub, fp, tok) {
		t.Error("token must not verify under a different key")
	}
	// garbage / tampered token fails closed.
	if VerifyApproval(pub, fp, "not-base64-!!") || VerifyApproval(pub, fp, "") {
		t.Error("malformed token must fail closed")
	}
}
