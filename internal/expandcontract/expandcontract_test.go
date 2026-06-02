package expandcontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/invariant"
)

func TestBuildReportGeneratesInvariantBackedTemplatesAndChecksORMProjects(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "rails/app/models/invoice.rb", `class Invoice < ApplicationRecord
  before_validation :dual_write_external_id
  def dual_write_external_id
    self.external_id ||= legacy_external_id
  end
end
`)
	writeFixture(t, root, "rails/db/migrate/20260101010101_expand_invoice_external_id.rb", `class ExpandInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    add_column :invoices, :external_id, :string, null: true
  end
end
`)
	writeFixture(t, root, "rails/db/migrate/20260101010202_backfill_invoice_external_id.rb", `class BackfillInvoiceExternalId < ActiveRecord::Migration[7.1]
  def up
    Invoice.where(external_id: nil).find_each { |invoice| invoice.update_all(external_id: invoice.legacy_external_id) }
  end
end
`)
	writeFixture(t, root, "rails/db/migrate/20260101010303_contract_invoice_external_id.rb", `class ContractInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    change_column_null :invoices, :external_id, false
    remove_column :invoices, :legacy_external_id
  end
end
`)
	writeFixture(t, root, "django/billing/models.py", `from django.db import models

class Invoice(models.Model):
    legacy_external_id = models.CharField(max_length=64)
    external_id = models.CharField(max_length=64, null=True)
    def save(self, *args, **kwargs):
        # dual_write_external_id keeps old writers compatible during expand.
        self.external_id = self.external_id or self.legacy_external_id
        super().save(*args, **kwargs)
`)
	writeFixture(t, root, "django/billing/migrations/0001_expand_invoice_external_id.py", `from django.db import migrations, models

class Migration(migrations.Migration):
    operations = [migrations.AddField("invoice", "external_id", models.CharField(max_length=64, null=True))]
`)
	writeFixture(t, root, "django/billing/migrations/0002_backfill_invoice_external_id.py", `from django.db import migrations

def backfill_external_id(apps, schema_editor):
    Invoice = apps.get_model("billing", "Invoice")
    for invoice in Invoice.objects.filter(external_id__isnull=True):
        invoice.external_id = invoice.legacy_external_id
        invoice.save(update_fields=["external_id"])

class Migration(migrations.Migration):
    operations = [migrations.RunPython(backfill_external_id)]
`)
	writeFixture(t, root, "django/billing/migrations/0003_contract_invoice_external_id.py", `from django.db import migrations, models

class Migration(migrations.Migration):
    operations = [migrations.AlterField("invoice", "external_id", models.CharField(max_length=64, null=False))]
`)

	report, err := BuildReport(validSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected expand/contract report to pass: %#v", report)
	}
	if report.Summary.Templates != 1 || report.Summary.Projects != 2 || report.Summary.ProjectsVerified != 2 || report.Hash == "" {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if got := report.Templates[0].Invariant.Kind; got != "unique" {
		t.Fatalf("expected invariant declaration to be embedded, got %q", got)
	}
	if !strings.Contains(RenderSQL(report), "ALTER TABLE invoices ALTER COLUMN external_id SET NOT NULL") {
		t.Fatalf("contract SQL missing from rendered template:\n%s", RenderSQL(report))
	}
	for _, check := range report.ORMChecks {
		if len(check.Evidence) < 6 {
			t.Fatalf("expected structural ORM evidence for %s: %#v", check.ProjectName, check)
		}
		for _, evidence := range check.Evidence {
			if filepath.IsAbs(evidence.Path) {
				t.Fatalf("evidence path must be stable and relative: %#v", evidence)
			}
		}
	}
}

func TestBuildReportRefutesMissingORMBackfillEvidence(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "bad-rails/app/models/invoice.rb", `class Invoice < ApplicationRecord
  before_validation :dual_write_external_id
  def dual_write_external_id
    self.external_id ||= legacy_external_id
  end
end
`)
	writeFixture(t, root, "bad-rails/db/migrate/20260101010101_expand_invoice_external_id.rb", `class ExpandInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    add_column :invoices, :external_id, :string, null: true
  end
end
`)
	writeFixture(t, root, "bad-rails/db/migrate/20260101010303_contract_invoice_external_id.rb", `class ContractInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    change_column_null :invoices, :external_id, false
  end
end
`)
	spec := validSpec()
	spec.ORMProjects = []ORMProjectSpec{{
		Name: "bad-rails", Ecosystem: "rails", Root: "bad-rails", Table: "invoices", Column: "external_id", LegacyColumn: "legacy_external_id",
	}}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.ORMChecks) != 1 {
		t.Fatalf("expected missing backfill to refute report: %#v", report)
	}
	if !contains(report.ORMChecks[0].Missing, "backfill_phase") {
		t.Fatalf("expected scanner-driven backfill miss, got %#v", report.ORMChecks[0].Missing)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.expand-contract/v1","name":"x","invariant_spec":{"version":"patchline.invariants/v1","name":"i","invariants":[]},"templates":[],"orm_projects":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "invoice external id expand/contract",
		InvariantSpec: invariant.Spec{
			Version: invariant.Version,
			Name:    "invoice invariants",
			Invariants: []invariant.Declaration{{
				ID: "invoice-external-id-unique", Kind: "unique", Table: "invoices", Column: "external_id",
			}},
		},
		Templates: []TemplateRequest{{
			ID: "invoice-external-id", InvariantID: "invoice-external-id-unique", LegacyColumn: "legacy_external_id", NewColumn: "external_id", BackfillExpression: "legacy_external_id",
		}},
		ORMProjects: []ORMProjectSpec{{
			Name: "rails", Ecosystem: "rails", Root: "rails", Table: "invoices", Column: "external_id", LegacyColumn: "legacy_external_id",
		}, {
			Name: "django", Ecosystem: "django", Root: "django", Table: "invoices", Column: "external_id", LegacyColumn: "legacy_external_id",
		}},
	}
}

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
