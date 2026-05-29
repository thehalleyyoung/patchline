import { prisma } from "./db";
import { Invoice } from "./models";

export async function listInvoices(customerId: string) {
  await prisma.invoice.findMany({ where: { customerId } });
  await Invoice.update({ status: "review" }, { where: { id: customerId } });
  await sequelize.query(`SELECT id, total_cents FROM invoices WHERE customer_id = ${customerId}`);
  await knex("invoice_events").where({ customer_id: customerId }).delete();
}
