from django.db import migrations, models


class Migration(migrations.Migration):
    operations = [
        migrations.AddField("invoice", "external_id", models.CharField(max_length=64, null=True))
    ]
