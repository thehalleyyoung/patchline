class InvoiceJob
  def perform(customer_id)
    Invoice.where(customer_id: customer_id).update_all(status: "review")
    ActiveRecord::Base.connection.execute <<~SQL
      INSERT INTO invoice_audits (invoice_id, reason)
      VALUES ('inv_1002', 'manual repair')
    SQL
  end
end
