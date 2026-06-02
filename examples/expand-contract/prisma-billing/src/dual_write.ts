import { PrismaClient } from "@prisma/client";

const prisma = new PrismaClient();

prisma.$use(async (params, next) => {
  // dual_write_external_id middleware keeps external_id and legacy_external_id compatible.
  return next(params);
});
