# Runbook: regional traffic failure

1. Confirm the failing region and origin pool from the SIEM / logs.
2. Reroute traffic to the nearest HEALTHY region (approved for OTHER regions; eu-west must fail over off-continent).
3. Notify the SOC on soc-incidents.
4. Open a ticket in servicenow.
5. Do NOT delete data. Data changes require a separate, human-approved change.
