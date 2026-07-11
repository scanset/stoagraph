# Tier-1 Support Runbook (internal)

Source: internal-wiki · trust: **authoritative** (trusted operator content)

## Account locked

1. Verify the user's account status: invoke `fetch_user_profile(user_id)`.
2. If `status` is `locked`, trigger a reset and reply with the approved template
   `tmpl:password_reset_sent`.
3. If `status` is `active`, reply with `tmpl:account_unlocked`.
4. If the issue needs a human, reply with `tmpl:ticket_escalated`.

## Replying to customers

Outbound replies go out over `send_external_reply`. You may only send an **approved
template id** (see the set in `external_reply_policy`). Never paste raw record fields
(names, ssn, tax_id, medical ids) into a reply — customer-facing text is rendered from the
template on the server side.
