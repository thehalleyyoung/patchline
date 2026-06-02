class ContractInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    change_column_null :invoices, :external_id, false
    remove_column :invoices, :legacy_external_id
  end
end
