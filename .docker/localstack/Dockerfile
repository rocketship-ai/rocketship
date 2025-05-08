FROM amazonlinux:2023.7.20250428.1

# install aws cli
RUN yum install -y aws-cli

# copy resource creation script
COPY create_resources.sh /create_resources.sh
RUN chmod +x /create_resources.sh

# run script to create resources
ENTRYPOINT ["/create_resources.sh"]
