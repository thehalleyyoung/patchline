from alembic import op

def upgrade():
    op.create_table("invoice_notes")
    op.execute("UPDATE invoices SET repair_marker = 'alembic' WHERE id = 'inv_1002'")
