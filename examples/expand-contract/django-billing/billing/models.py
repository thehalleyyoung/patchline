from django.db import models


class Invoice(models.Model):
    legacy_external_id = models.CharField(max_length=64)
    external_id = models.CharField(max_length=64, null=True)

    def save(self, *args, **kwargs):
        # dual_write_external_id keeps legacy and new writers compatible.
        self.external_id = self.external_id or self.legacy_external_id
        super().save(*args, **kwargs)
