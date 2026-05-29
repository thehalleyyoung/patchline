class BillingDao {
  void deleteDrafts(java.sql.Connection connection) throws Exception {
    connection.prepareStatement("""
      DELETE FROM invoices
      WHERE status = 'draft'
    """);
    repository.find();
  }
}
