# Clustered Storage application
This is a simple clustered storage application which faces the problems of distributed systems such as leader election, replication, and fault tolerance.

## Fixes
- [x] Remove "io/ioutil" package from imports since it has been deprecated.
- [x] Refactor. Remove unnecessary variables.

## Features/Improvements
- [x] Add a reference to the leader so that the follower can send the leader's address to the client