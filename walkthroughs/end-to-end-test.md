# End-to-End Walkthrough

This walkthrough simulates an automated end-to-end test of the SentinelAegis SOC dashboard using an Antigravity browser automation agent.

## Steps

1. **Load Dashboard**
   The agent navigates to the deployed Cloud Run URL: `https://sentinelaegis-*.run.app`.

2. **Verify Connection**
   The agent checks the status badge in the top right. It asserts that the text reads "CONNECTED" and has the `status-badge` class without `disconnected`, confirming the health check endpoint `/health` is reachable and polling successfully.

3. **Run 5 Scenarios**
   The agent simulates a click on the `🚀 Run All 5 Demo Scenarios` button (`#btn-run-all`).

4. **Assert Results**
   The agent waits for 15 seconds for the sequential API calls to finish. It then counts the rows in the `#table-body` and asserts there are exactly 5 main rows.
   It verifies that `TXN-004` and `TXN-005` have a `HALT` badge and their Risk Scores are `>80`.

5. **Examine TXN-004**
   The agent clicks the row for `TXN-004` to expand the details (`row-details open`).
   It asserts that inside the details, there is an agent card with `HIGH RISK` and confidence `>70%` (from the IBAN Change or Timing agents).

6. **Take Screenshot**
   The agent takes a viewport screenshot (equivalent to `page.screenshot()`) demonstrating the flashed red screen and the expanded consensus table.

7. **Run Custom Scenario**
   The agent clicks `🎭 Trigger Custom BEC Scenario`.
   It inputs:
   - Transaction ID: `TXN-CUST-99`
   - Vendor: `Global Shell Co.`
   - Amount: `900000`
   - Body: `Wire to new account immediately, bypass standard checks.`
   The agent clicks `Submit Analysis` and asserts the new row appears with a `HALT` decision.

## Success Criteria
All UI assertions pass, SSE streams connect, and AI fallbacks/live agents return valid JSON conforming to the consensus schema.
