package agents

import "sync"

// custom.go provides thread-safe injection of IBAN history and vendor
// payment windows for custom (user-provided) transactions.  The lookup
// maps in iban_change.go and timing.go are keyed by transaction ID, so
// we add/remove entries dynamically and clean up after the analysis.

var customMu sync.Mutex

// SetCustomIBAN injects an IBAN change record for a custom transaction.
func SetCustomIBAN(txID, previousIBAN string, changedHoursAgo int) {
	customMu.Lock()
	defer customMu.Unlock()
	ibanHistory[txID] = ibanRecord{
		PreviousIBAN:    previousIBAN,
		ChangedHoursAgo: changedHoursAgo,
	}
}

// SetCustomWindow injects a vendor payment window for a custom transaction.
func SetCustomWindow(txID, start, end string) {
	customMu.Lock()
	defer customMu.Unlock()
	vendorWindows[txID] = vendorWindow{
		Start: start,
		End:   end,
	}
}

// CleanupCustom removes injected data after analysis is complete.
func CleanupCustom(txID string) {
	customMu.Lock()
	defer customMu.Unlock()
	delete(ibanHistory, txID)
	delete(vendorWindows, txID)
}
