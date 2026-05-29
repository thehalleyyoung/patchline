UPDATE TOP (10) [dbo].[Invoices] SET total_cents = 0;

DELETE TOP (5) FROM [dbo].[LedgerEntries];
