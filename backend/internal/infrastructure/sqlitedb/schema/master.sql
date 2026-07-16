CREATE TABLE IF NOT EXISTS "unis" ("id" INTEGER PRIMARY KEY AUTOINCREMENT, "num" INTEGER, "dbhost" TEXT, "dbuser" TEXT, "dbpass" TEXT, "dbname" TEXT, "uniurl" TEXT);
CREATE TABLE IF NOT EXISTS "coupons" ("id" INTEGER PRIMARY KEY AUTOINCREMENT, "code" TEXT, "amount" INTEGER, "used" INTEGER, "user_uni" INTEGER, "user_id" INTEGER, "user_name" TEXT);
