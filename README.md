dockerConnector for DomeOS
===

update 2015-06-01: 

docker exec process would be killed on channel closing.

## Instruction

This is the client for docker container ssh login.

## Running

```bash
sudo ./dockerConnector -d
```

## Login remote container

```bash
ssh <container ID>@<remote address> -p 2222
```

## Remark

We've disabled password authentication by modifying ssh package in Godeps for convenient container login. If you want to verify password while login, please change ssh packages in Godeps to official packages(https://github.com/golang/crypto)  and modify passwordCallback in connector.go.
