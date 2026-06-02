class Invoice < ApplicationRecord
  before_validation :dual_write_external_id

  def dual_write_external_id
    self.external_id ||= legacy_external_id
  end
end
