class ContractInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    change_column_null :invoices, :external_id, false
  end
end
