# Download arXiv dataset

```bash
pip install s3cmd
s3cmd --configure

# https://github.com/armancohan/arxiv-tools/tree/master

# https://medium.com/@shamnad.p.s/how-to-create-an-s3-bucket-and-aws-access-key-id-and-secret-access-key-for-accessing-it-5653b6e54337
# https://us-east-2.console.aws.amazon.com/console/home?region=us-east-2
# https://us-east-1.console.aws.amazon.com/iam/home?region=us-east-1#/security_credentials

s3cmd get --requester-pays s3://arxiv/src/arXiv_src_manifest.xml ./arXiv_src_manifest.xml
```
