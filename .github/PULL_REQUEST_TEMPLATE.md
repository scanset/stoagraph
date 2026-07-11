## What and why

<!-- What changes, and what problem it solves. Link the issue if there is one. -->

## Checklist

- [ ] `tools/check.sh` passes (fmt · vet · test · **ARCHITECTURE** · typecheck · index · hygiene)
- [ ] Commits are signed off (`git commit -s`) — DCO
- [ ] Added a `// file-kw:` marker to any new file and re-ran `tools/index.sh`
- [ ] A bug fix has a test that **fails without it**
- [ ] Docs updated if behaviour changed (**SECURITY.md if a guarantee moved**)
- [ ] `tools/sbom.sh` re-run if dependencies or a Dockerfile changed

## Does this change what an agent is allowed to do?

<!--
If yes: say what stops a hijacked or prompt-injected model from abusing it.
"The model wouldn't do that" is not an answer — the entire premise is that it might.
If no: say "no".
-->
