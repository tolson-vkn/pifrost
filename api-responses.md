## API responses

Useful for debugging, error handling. These come from pihole.

#### Get current DNS

```
curl http://10.1.1.5/admin/api.php?action=get&auth=TOKEN&customdns=
```

Response:

```
{"data":[["foo.tolson.io","10.1.1.1"],["bar.tolson.io","10.1.1.1"],["tip.tolson.io","10.1.1.5"]]}[]
```

#### Create record

```
curl http://10.1.1.5/admin/api.php?action=add&auth=TOKEN&customdns=&domain=google.tolson.io&ip=8.8.8.8
```

Response:

```
{"success":true,"message":""}{"FTLnotrunning":true}
```

#### Attempt duplicate

```
curl http://10.1.1.5/admin/api.php?action=add&auth=TOKEN&customdns=&domain=google.tolson.io&ip=8.8.8.8
```

Response:

```
{"success":false,"message":"This domain already has a custom DNS entry for an IPv4"}[]
```


#### Delete record that exists:

```
curl http://10.1.1.5/admin/api.php?action=delete&auth=TOKEN&customdns=&domain=domain&ip=8.8.8.8
```

Response:

```
{"success":true,"message":""}{"FTLnotrunning":true}
```


#### Delete record that doesn't exist:

```
curl http://10.1.1.5/admin/api.php?action=delete&auth=TOKEN&customdns=&domain=domain&ip=8.8.8.8
```

Response:

```
{"success":false,"message":"This domain\/ip association does not exist"}[]
```
