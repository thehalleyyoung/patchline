from django.db import migrations, models


class Migration(migrations.Migration):
    dependencies = [
        ("blog", "0006_backfill_slug"),
    ]

    operations = [
        migrations.AddField(
            model_name="post",
            name="slug",
            field=models.SlugField(null=False),
        ),
    ]
