import { getRepository } from "typeorm";

export async function closeInvoices() {
  await getRepository(Invoice).update({ status: "open" }, { status: "closed" });
}
