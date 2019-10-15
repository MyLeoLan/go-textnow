# go-textnow  [![Build Status](https://travis-ci.org/OmarElGabry/go-textnow.svg?branch=master)](https://travis-ci.org/OmarElGabry/go-textnow)
A phone service built using Go, gRPC &amp; gRPC Gateway, MySQL &amp; MongoDB, Redis, zap, and OpenCensus.

# Overview
![Overview](https://raw.githubusercontent.com/OmarElGabry/go-textnow/master/assets/use-cache.png)

## Features
1.  Two gRPC services and a gRPC Gateway sitting infront of them, acts as a proxy, and translates a RESTful HTTP API into gRPC.
2.  OpenCensus for metrics and tracing exporting to Datadog, Jaeger, etc.
3.  All gRPC services and gateway are dockerized, including a container for testing.
4.  The repository is integrated with TravisCI. On each commit, will build and run all the tests. TravisCI uses Docker compose.
5.  Bazel for building the services and deploying them off to Kubernestes.

## Install and Run
#### Docker compose (development)
Make sure to edit and rename `.env.example`.

```
docker-compose up
```
Docker compose runs all containers with a single command and creates a shared network between these containers so communication between services is done using server name instead of the IP address.

It also mounts the whole application directory and so to avoid re-building the containers.

#### Bazel (development / production)
Make sure to edit and rename `deployment/k8s/secrets.example.yaml`
```
# deploy the secrets
kubectl apply -f deployment/k8s/secrets.yaml

# deploy the services (phonebook, sms, and gateway)
bazel run //cmd/phonebook:phonebook_k8s.apply --incompatible_depset_is_not_iterable=false --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64
bazel run //cmd/sms:sms_k8s.apply --incompatible_depset_is_not_iterable=false --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64
bazel run //cmd/gateway:gateway_k8s.apply --incompatible_depset_is_not_iterable=false --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64
```

_The flag `--incompatible_depset_is_not_iterable=false` is due to this [issue/bug](https://github.com/bazelbuild/rules_k8s/issues/248),
while `--platforms=@io_bazel_rules_go//go/toolchain:linux_amd64` is to force building the binary for Linux since it will run in a Linux container_.

**As far as compiling the proto files is concerned**, bazel can do it, but it is not implemented at the current moment. Alternatively, run the scripts to compile the proto files manually. Bazel, or more specifically, Gazelle, doesn't like having proto files with different package names in the same directory. Every folder represent a package. Only files related to that package should exist.

## Folder structure
- `/cmd` contains the main.go files for each service. Each service reside in a folder
- `/internal` contains the packages for this application.
	- `/phonebook` contains all files related to phonebook service.
	- `/sms` contains all files related to sms service.
	- `/pkg` contains all shared, common packages such as mysql, redis, logger, etc.
- `/api` contains proto files for all services
- `/tests` contains all tests files for all services
- `/build` contains the `Dockerfile` for each service in a folder
- `/deployment/k8s` contains the k8s yaml files

## Use cases
We have two services: phonebook and sms.
### Phonebook
It consists of 3 methods to find, reserve, and assign a phone number.

**FindOne**
Finds if the given phone number exists or not. 
    
**Reserve**
Reserves 5 (unassigned) phone numbers with a given area code and return them back to the user to choose one of them.

**Assign**
Assigns the selected number to the user. It is called after Reserve method to carry on the phone number assignment.

### SMS
It consists of 2 methods to send a single and multiple SMSs.

**SendOne**
Sends a single sms given the phone numbers _from_ and _to_ and the sms content. This method must be idempotent. It must be safe to retry sending the same SMS and will be sent only once.

**SendMany**
Sends many SMSs in one request.

This is used when one SMS contains long text (exceeds limit of 1 sms), and so the client will chunck it up, and split it into smaller SMSs and send them in one request.

## Assumptions
- For FindOne, it is a normal siutation to get requests where phone number doesn't exist.
- On Reserve or Assign, assume that user already exists.

## Constraints 
#### Performance
- FindOne: 10M/day ~= 100/second
- Reserve: 10H/minute ~= 1/second 
- Assign:  10H/minute ~= 1/second  (same as Reserve)
- SendOne: 50M/day ~= 500/second
	- SMSs are likely to be sent more often vs phone calls.
- SendMany: 50/second 
	- The ratio is, for each 500 normal sms, there are 50 big sms.
	- The average number of chuncked smss in one request is 2.
#### Data
For simplicity, currently exists 1M users, 10,000 (unassigned) phone numbers, and 5M sms.

For phonebook service, only the user's phone number is updated. Each phone number can be, say, ~15 bytes, in total = 15 * 86400 _(seconds per day)_   ~= 1.2MB per day.

For sms service, each sms contains _from_ & _to_ phone numbers, the sms content and the idempotency key. Each of the from and to is ~15 bytes, while the sms content is ~160 bytes and idempotency key is 16 bytes. In total we get (15 * 2) + 160 + 16 = 206 bytes per sms, and 206 * 500 * 86400 = 8.8 GB per day.

## High-Level Design
![HLD](https://raw.githubusercontent.com/OmarElGabry/go-textnow/master/assets/hld.png)

#### FindOne
It is just a simple query to the database, a relational database. 

Table `phonebook` contains all user data including the phone number assigned for each user if exists.

```sql 
SELECT user_id FROM phonebook WHERE phone_number=?
```
_A prepared statements is created under the coverd and then executing it with the parameters._

Since we are to query the user's phone number, having an index around `phone_number` column improves the query performance. Moreover, partitioning based on user id or phone number helps in reducing the number of rows to scan and essential if all data can't fit in one server.

REST API:
```
curl https://localhost:8080/phonebook/find/+18823672995
```

Response:
```
{ "exists": true }
```
#### Reserve
Reserves 5 (unassigned) phone numbers and requires the user to choose one of them.

Table `un_assigned_numbers` has a list of all phone numbers that are available to be assigned to any user. Each row has the phone number, its area code, status and other columns as explained below.

(1) UPDATE 5 phone numbers with the given `areaCode`, assign `refID` and `status` to INUSE.
```sql
UPDATE un_assigned_numbers SET ref_id=?, timestamp=?, status='INUSE' WHERE area_code=? AND status='AVAILABLE' LIMIT 5;
```
_This assumes that the database engine ensures that a row is updated by one process at a time. And so, avoid reserving the same phone numbers by two different concurrent threads._

(2) SELECT those 5 numbers given the `refID`
```sql
SELECT phone_number FROM un_assigned_numbers WHERE ref_id=?
```

Since we will be quering based on `ref_id` & `area_code` columns, it is useful to have indexes around these columns.

REST API:
```
curl -d '{"areaCode": 613}' -H "Content-Type: application/json" -X POST http://localhost:8080/phonebook/reserve
```

Response:
```
{ "refId": "64150fe2-b0c5-4bd4-a00e-399872656982",
  "phoneNumbers": [
	  "+16131513601", 
	  "+16137343307",
	  "+16137692043",
	  "+16139802378",
	  "+16138874322"  
  ]
}
```

#### Assign
Assigns the selected number to the user. It is called after Reserve method to carry on the phone number assignment.

It updates the user's row in `phonebook` table.

(1) Check if the given `refId` and `phoneNumber` actually exist.

```sql
SELECT phone_number FROM un_assigned_numbers WHERE ref_id=? AND phone_number=?
```

(2) Update the _not_ selected phone numbers and clear their `refId`, `timestamp`, and `status`.

```sql
UPDATE un_assigned_numbers SET status='AVAILABLE', ref_id='', timestamp=NULL WHERE ref_id=? AND phone_number!=?
```
(3) Inside a transaction:
   1. Delete the selected number from ``un_assigned_numbers`` table.
```sql
	DELETE FROM un_assigned_numbers WHERE ref_id=?
```
   2. Assign the selected number to the given `userId` in `phonebook` table.
```sql
	UPDATE phonebook SET phone_number=? WHERE user_id=?
```
   3. Commit the transaction.

_Instead of using transactions, another way of doing it is to, rather than deleting, just flag the phone numebr as "ASSIGNED" so other users won't reserve it. Then a background process will re-try to do it later if failed and delete it from ``un_assigned_numbers`` table when successful_.

_As requests to `Assign` might fail for whatever reason, a background script runs every 10 minutes which searches for the phone numbers that have been reserved but weren't assigned, and resets the columns such as heir `refID`, `timestamp`, and `status`_.

REST API:
```
curl -d '{"userId": 123, "refId": "64150fe2-b0c5-4bd4-a00e-399872656982", "phoneNumber": "+16131513601"}' -H "Content-Type: application/json" -X POST http://localhost:8080/phonebook/assign
```

Response:
```
{ "assigned": true }
```

#### SendOne
Sends a single sms. For sms, we'll use a NoSQL database such as MongoDB. 

_**NOTE**_ Sending sms is done by just inserting it to the database. Nothing will be actually sent.

Each sms has phone numbers `from` and `to` and the sms `content`. It can be as simple an Insert command to `sms` collections in MongoDB. It will **also** check for the existence of `from` and `to` phone numbers by invoking `FindOne` of phonebook service.

To make sure it is idempotent, we need first to check if idempotency key existence. There are different ways of doing this, one approach:
1. For each request, insert the idempotent key. 
2. Assuming we have an index (unqiue) around the idempotent key, we'll either get "already exists" error or success. If it already exists, return "SMS has been sent already".
3. Otherwise, continue execution, and send sms.
4. On failure, delete the idempotent key so that the same sms can be sent again in the future.

To avoid concurrent requests from sending the same sms, it is assumed that only one insert operation (in step 1 above) will be executed by MongoDB at a time.

REST API:
```
curl -d '{"sms": {"idempotencyKey": "lmkasdlamslk123sxaxad2", "fromPhoneNumber": "+16135550172", "toPhoneNumber": "+16135550172", "content": "hi, how are you?"}}' -H "Content-Type: application/json" -X POST http://localhost:8080/sms/send/one
```

Response:
```
{ "sent": true }
```

_There are some assumptions on how the client generates the idempotency keys. For example, it must be unique and make sure to use the same one on re-try_.

#### SendMany
It relies on calling SendOne method for each sms. 

And so, for every sms, call `SendOne`. If any errors, add it to errors array and keep looping. When done through all sms, return an array of errors if any.

To avoid having the client to wait until all the SMSs have been sent, we can use the idea of "Tracking ID". And so,
1. For each request, create a record in the database where its status is "IN_PROGRESS", and send back the client a request tracking id.
2. The SMSs will be sent at the background, and so terminating the reqeust as early as possible.
3. This request tracking id can be later used to know about the status of the sms, and if any errors, when we're done sending them.

REST API:
```
curl -d '{"sms": {"idempotencyKey": "lmkasdlamslk123sxaxad2", "fromPhoneNumber": "+16135550172", "toPhoneNumber": "+16135550172", "content": "hi, how are you?"}} {"sms": {"idempotencyKey": "lmkasdlamslk123sxaxad2", "fromPhoneNumber": "+16135550172", "toPhoneNumber": "+16135550172", "content": "hope you are doing well."}}' -H "Content-Type: application/json" -X POST http://localhost:8080/sms/send/many
```

Response:
```
{ "errors": [] }
```

_For HTTP API, newline-delimited JSON is used for streaming. SMSs are sent one by one in a stream. This is done thanks to the grpc-gateway_.

## Adding cache
![Use cache](https://raw.githubusercontent.com/OmarElGabry/go-textnow/master/assets/use-cache.png)

The database is the slowest piece in the application. And so we avoid database calls and use cache instead.  

Facts about the nature of the application that can simplify the work. For example, un assigned phone numbers are not that many and can all reside in a cache. If the data is huge, then we can store only frequently used numbers in cache as a way to optimize popular queries.

A solution is to use Redis; an in-memory cache that supports different data structures, presistence and replication.

It is a good idea to have two Redis instances for two different eviction policies. For `un_assigned_numbers`,  we can no eviction policy, while LRU policy for users' phone numbers (`phone_number` column in `phonebook` tables).

#### FineOne
1. Check cache: `Cache[phoneNumber]`
2. If not exists (cache miss), get phone number from database.
3. If not exists in database, return "Not exists". 
4. If found, update the cache: `Cache[phoneNumber]` = true, and return "Exists".

To avoid having multiple cache miss resulting from multiple concurrent requests (cache stampede), there are a couple of options: 
- Locking: A typical solution is to lock each request until we update the cache if cache miss. And so next request will find it in the cache.
- Warm Up: Initially and Periodically. Initially, warm up the cache when it boots. Periodically, re-insert/update the cache periodically.
- Debounce: Allow multiple requests to come in. If cache miss, initiate one database query, and force all other requests asking for the same data to wait for that same query. When query is done, the result is made available to all the requests waiting for it.
- A serial queue: If cache miss, we update the cache. All subsequent requests will then find the data in the cache. This only works if it Ok to return the query result async.

#### Reserve

1. Pull 5 from `Cache[AC]`, where AC is the given areaCode
_To avoid multiple requests reserving the same phone numbers: Redis is actually single-threaded, and so only one command at a time is executed_.
2. Check if count of pulled phone numbers is less than 5
3. Assign to `Cache[refID]` = [...phone numbers...]

_To avoid processing the same request (i.e. user hit the button twice), idemptoency key should be used as in `SMS@SendOne`_.

#### Assign

1. Check if `refId` and the selected phone number are valid by checking `Cache[refID]` & `Cache[refID][phoneNumber]`
2. Delete `refId` key from the Cache.
3. Re-push the un-selected numbers into `Cache[AC]`
4. Assign the selected number in database in `phonebook` table.
5. Update the cache with the newly assigned number.

The UPDATE statement in step 4 takes time because it hits the database. This can be improved by storing the newly assigned phone number (`phone_number` in `phonebook` table) in the cache and update the database at the background. For this to work, we need to use async queue to carry on storing data in the database, re-try on failure, etc.
