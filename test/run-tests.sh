#!/usr/bin/env bash

cd test1 || exit 1
../test.sh
cd .. || exit 1

cd test2 || exit 1
../test.sh