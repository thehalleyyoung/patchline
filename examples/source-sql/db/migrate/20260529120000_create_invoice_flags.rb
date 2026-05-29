class CreateInvoiceFlags < ActiveRecord::Migration[7.1]
  def change
    create_table :invoice_flags do |t|
      t.string :invoice_id
    end
  end
end
