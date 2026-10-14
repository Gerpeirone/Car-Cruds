db.getSiblingDB("fleet").createUser({
  user: "fleet", pwd: process.env.MONGO_FLEET_PASSWORD,
  roles: [{role: "readWrite", db: "fleet"}]
});
