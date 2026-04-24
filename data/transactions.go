package data

// Transaction represents a payment request under review.
type Transaction struct {
	ID           string  `json:"id"`
	Vendor       string  `json:"vendor"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	IBAN         string  `json:"iban"`
	EmailSubject string  `json:"email_subject"`
	EmailSender  string  `json:"email_sender"`
	EmailText    string  `json:"email_text"`
	RequestedAt  string  `json:"requested_at"` // HH:MM
	Status       string  `json:"status"`       // pending, approved, halted
}

// DemoTransactions returns 5 hardcoded transactions for the demo.
// 2 safe, 1 borderline, 2 high risk.
func DemoTransactions() []Transaction {
	return []Transaction{
		{
			ID:           "TXN-001",
			Vendor:       "CloudVault Hosting",
			Amount:       32400,
			Currency:     "USD",
			IBAN:         "US44JPMC021000089012",
			EmailSubject: "Monthly Invoice #CV-2026-04",
			EmailSender:  "invoices@cloudvault-hosting.com",
			EmailText: `Dear ACME Corporation Accounts Payable,

Please find attached your monthly invoice for April 2026 hosting services.

Invoice: #CV-2026-04
Amount: $32,400.00
Due Date: April 30, 2026
Payment Terms: Net 30

Services rendered:
- Enterprise cloud hosting (April 1–30)
- 99.97% uptime SLA met
- 4.2TB bandwidth consumed
- 24/7 priority support included

Our banking details remain unchanged. Please reference invoice number CV-2026-04 in your payment.

Thank you for choosing CloudVault. As always, reach out to your account manager Maria Torres (maria.t@cloudvault-hosting.com) with any questions.

Warm regards,
CloudVault Billing Team`,
			RequestedAt: "10:15",
			Status:      "pending",
		},
		{
			ID:           "TXN-002",
			Vendor:       "ADP Payroll Services",
			Amount:       87500,
			Currency:     "USD",
			IBAN:         "US66WELL019283746510",
			EmailSubject: "Bi-weekly Payroll Run Confirmation",
			EmailSender:  "payroll@adp.com",
			EmailText: `Hello ACME Finance Team,

This is a confirmation of your scheduled bi-weekly payroll run for April 15–28, 2026.

Total Payroll Amount: $87,500.00
Employees Covered: 47
Direct Deposits: 47 of 47
Tax Withholdings: Calculated and filed

The payroll file has been submitted and will be processed on April 28. No action is required on your end.

If you have any questions, please contact your dedicated ADP representative, James Holbrook, at j.holbrook@adp.com or (800) 555-0142.

Best regards,
ADP Payroll Processing`,
			RequestedAt: "09:30",
			Status:      "pending",
		},
		{
			ID:           "TXN-003",
			Vendor:       "Partner Logistics Intl",
			Amount:       124000,
			Currency:     "USD",
			IBAN:         "GB82WEST12345698765432",
			EmailSubject: "Invoice #PL-29847 — Payment Due This Week",
			EmailSender:  "d.chen@partnerlogistics.net",
			EmailText: `Hello Finance Team,

Please find attached our updated invoice #PL-29847 for the Q2 logistics contract. The total is $124,000 as agreed in our March SOW.

Kindly note our banking details have been updated following our recent transition to a UK-based treasury operation. The new IBAN is reflected on the invoice. We'd appreciate payment by end of this week if possible, as we have our own vendor commitments to meet.

If you have any questions about the banking change, feel free to reach out to our treasury team at treasury@partnerlogistics.net.

Thank you for the continued partnership.

Best,
David Chen
Accounts Receivable Manager
Partner Logistics International`,
			RequestedAt: "17:45",
			Status:      "pending",
		},
		{
			ID:           "TXN-004",
			Vendor:       "Greenfield Solutions",
			Amount:       847000,
			Currency:     "USD",
			IBAN:         "GB29NWBK60161331926819",
			EmailSubject: "URGENT — Confidential Wire Transfer Required Today",
			EmailSender:  "james.whitfield.ceo@acme-corp.mail.com",
			EmailText: `Hi Sarah,

I need you to process an urgent wire transfer of $847,000 to Greenfield Solutions. They've updated their banking details — the new IBAN is listed in the attached invoice.

This is related to the Meridian acquisition we've been working on confidentially. It is absolutely critical that this goes out before 7pm today. We will lose the deal if this payment is not received by tomorrow morning.

Please do NOT discuss this with anyone else on the team — the deal is still under NDA and any leak could jeopardize the entire transaction. I'm in back-to-back board meetings all day so email is the only way to reach me right now.

I trust you to handle this with the urgency it requires. Confirm once sent.

Best regards,
James Whitfield
CEO, ACME Corporation
james.whitfield@acme-corp.com`,
			RequestedAt: "18:47",
			Status:      "pending",
		},
		{
			ID:           "TXN-005",
			Vendor:       "Meridian Holdings",
			Amount:       1250000,
			Currency:     "USD",
			IBAN:         "CH9300762011623852957",
			EmailSubject: "RE: Acquisition Escrow — WIRE IMMEDIATELY",
			EmailSender:  "legal-escrow@meridian-holdings.co",
			EmailText: `Sarah,

Following up on James' email. The escrow agent is expecting the $1,250,000 wire TODAY. I've attached the updated wiring instructions — please use the Swiss account listed.

This is the final tranche of the Meridian deal. Any delay past 5pm CET will trigger the penalty clause in Section 14.2 of the SPA. The board has already approved this — do NOT route through the normal approval chain as it will cause delays we cannot afford.

I've copied James but please do not reply-all. Keep this between us until the deal closes.

Thanks,
Robert Park
General Counsel, ACME Corporation`,
			RequestedAt: "23:14",
			Status:      "pending",
		},
	}
}
