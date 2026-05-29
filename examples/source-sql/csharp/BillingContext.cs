class BillingContext {
  void Read(BillingDbContext context) {
    context.Invoices.Where(invoice => invoice.Status == "open");
    context.Database.ExecuteSqlRaw("UPDATE invoices SET status = 'review' WHERE status = 'open'");
  }
}
