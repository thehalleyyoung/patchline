from django.db import migrations, models


class Migration(migrations.Migration):
    operations = [
        migrations.AlterField("invoice", "external_id", models.CharField(max_length=64, null=False))
    ]
