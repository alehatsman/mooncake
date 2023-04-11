#!/bin/bash

git tag --delete latest;
git push --delete origin latest;
gh release delete latest -y;
git tag latest;
git push origin latest;
