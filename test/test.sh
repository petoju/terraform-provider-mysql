#!/usr/bin/env bash

export TF_VAR_MYSQL_ROOT_USER="root"
export TF_VAR_MYSQL_ROOT_PASSWORD="$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 20 | head -n 1)"
container1=$(docker run --rm --name test-mysql8.4.5-1 -e MYSQL_ROOT_PASSWORD=$TF_VAR_MYSQL_ROOT_PASSWORD -d -p 3307:3306 mysql:8.4.5 mysqld --mysql-native-password=ON)
if ! echo "$container1" | grep -Eq '^[0-9a-f]+$'; then
    exit 1
fi
container2=$(docker run --rm --name test-mysql8.4.5-2 -e MYSQL_ROOT_PASSWORD=$TF_VAR_MYSQL_ROOT_PASSWORD -d -p 3308:3306 mysql:8.4.5 mysqld --mysql-native-password=ON)
if ! echo "$container2" | grep -Eq '^[0-9a-f]+$'; then
    exit 1
fi


echo "Waiting for MySQL to become available..."
until docker exec "$container1" mysqladmin ping -h"localhost" -P3306 --silent; do
    sleep 1
done
until docker exec "$container2" mysqladmin ping -h"localhost" -P3306 --silent; do
    sleep 1
done


terraform init
terraform_code=$?
if [ $terraform_code -eq 0 ]; then
    terraform apply -auto-approve
    terraform_code=$?
fi

docker container stop $container1
docker container stop $container2
exit $terraform_code