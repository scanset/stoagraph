# Runbook: regional traffic failure

1. Confirm the failing region and origin pool from the SIEM / incident logs.
2. EU customer traffic must stay in the EU (data residency). Reroute the failing EU region's
   failover traffic to the sanctioned EU failover region.
3. Notify the SOC on soc-incidents.
4. Open a ticket in servicenow.
5. Do NOT delete data. Data changes require a separate, human-approved change.
