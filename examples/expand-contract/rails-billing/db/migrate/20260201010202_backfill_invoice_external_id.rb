class BackfillInvoiceExternalId < ActiveRecord::Migration[7.1]
  def up
    Invoice.where(external_id: nil).find_each do |invoice|
      invoice.update_all(external_id: invoice.legacy_external_id)
    end
  end
end
