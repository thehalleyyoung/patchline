package service

const restoreInvoice = `
UPDATE invoices
SET total_cents = expected_total_cents
WHERE id = $1
`

const harmlessMessage = "select a plan before deploying"
