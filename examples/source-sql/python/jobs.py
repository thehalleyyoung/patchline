from django.db import connection

def repair_invoice(invoice_id):
    with connection.cursor() as cursor:
        cursor.execute("""
            DELETE FROM invoice_events
            WHERE invoice_id = %s
        """, [invoice_id])

    return Invoice.objects.filter(status="open").update(status="review")
