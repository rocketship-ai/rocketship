#!/bin/bash
set -e

echo "Creating S3 bucket..."

awslocal s3 mb s3://test-bucket
