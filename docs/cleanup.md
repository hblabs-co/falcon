# System Reset — Clean State

Resets CVs, users, NATS streams, Qdrant vectors, and MinIO files.
Keeps the projects you choose in MongoDB.

---

## 1. MongoDB — delete CVs and users

```bash
docker exec -it falcon-mongo-1 mongosh falcon --eval "
  db.cvs.deleteMany({});
  db.users.deleteMany({});
  print('cvs deleted:', db.cvs.countDocuments());
  print('users deleted:', db.users.countDocuments());
"
```

## 2. MongoDB — delete specific projects

Replace the IDs with the ones you want to remove:

```bash
docker exec -it falcon-mongo-1 mongosh falcon --eval "
  db.projects.deleteMany({ id: { \$in: ['ID_1', 'ID_2', 'ID_3'] } });
  print('projects remaining:', db.projects.countDocuments());
"
```

To delete ALL projects except the ones you want to keep:

```bash
docker exec -it falcon-mongo-1 mongosh falcon --eval "
  db.projects.deleteMany({ id: { \$nin: ['KEEP_ID_1', 'KEEP_ID_2'] } });
  print('projects remaining:', db.projects.countDocuments());
"
```

---

## 3. NATS JetStream — purge all streams
- brew install nats-io/nats-tools/nats

```bash
nats stream purge PROJECTS --force -s nats://localhost:4222
nats stream purge CVS --force -s nats://localhost:4222
nats stream purge MATCHES --force -s nats://localhost:4222
```

Verify streams are empty:

```bash
nats stream report -s nats://localhost:4222
```

---

## 4. Qdrant — delete all vectors

```bash
curl -X POST http://localhost:6333/collections/cvs/points/delete \
  -H 'Content-Type: application/json' \
  -d '{"filter": {}}'
```

Verify the collection is empty:

```bash
curl http://localhost:6333/collections/cvs
```

The `points_count` should be `0`.

---

## 5. MinIO — delete all CV files

Option A — via the web console (easiest):
1. Open http://localhost:9001
2. Login: `minioadmin` / `minioadmin`
3. Go to **Buckets → cvs → Browse**
4. Select all → Delete

Option B — via CLI:

```bash
docker exec -it falcon-minio-1 sh -c "mc alias set local http://localhost:9000 minioadmin minioadmin && mc rm --recursive --force local/cvs"
```

---

## Verify clean state

```bash
docker exec -it falcon-mongo-1 mongosh falcon --eval "
  print('projects:', db.projects.countDocuments());
  print('cvs:',      db.cvs.countDocuments());
  print('users:',    db.users.countDocuments());
"
```

```bash
curl -s http://localhost:6333/collections/cvs | grep points_count
```

```bash
docker exec -it falcon-nats-1 nats stream report
```
