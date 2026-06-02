from django.db import migrations


def backfill_external_id(apps, schema_editor):
    Invoice = apps.get_model("billing", "Invoice")
    for invoice in Invoice.objects.filter(external_id__isnull=True):
        invoice.external_id = invoice.legacy_external_id
        invoice.save(update_fields=["external_id"])


class Migration(migrations.Migration):
    operations = [
        migrations.RunPython(backfill_external_id)
    ]
