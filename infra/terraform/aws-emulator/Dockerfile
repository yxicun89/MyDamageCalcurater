# AWS Emulator のイメージ(公式は既製イメージを配らず、この形の Dockerfile を案内している。
# AWS Emulator User Guide「Getting started with Docker」)。版は固定し、上げるときは scripts/tf-aws-emulator.sh の
# AWS_EMULATOR_VERSION・AWS_EMULATOR_SHA256_* と一緒に変える。Linux なら Docker を使わず Debian/RPM パッケージを直接入れてもよい(README)。
FROM public.ecr.aws/amazonlinux/amazonlinux:2023-minimal
ARG AWS_EMULATOR_VERSION
ARG AWS_EMULATOR_ARCH
ARG AWS_EMULATOR_SHA256
# --checksum で、取得した RPM が scripts/tf-aws-emulator.sh に書いたハッシュと一致することを確かめる
ADD --checksum=sha256:${AWS_EMULATOR_SHA256} https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/aws-emulator-${AWS_EMULATOR_VERSION}-1-${AWS_EMULATOR_ARCH}.rpm /tmp/aws-emulator.rpm
RUN rpm -i /tmp/aws-emulator.rpm && rm /tmp/aws-emulator.rpm \
    && mkdir -p /opt/emulator && chown 1000 /opt/emulator
USER 1000
ENV HOME=/opt/emulator
EXPOSE 9297
ENTRYPOINT ["aws-emulator", "-b", "0.0.0.0"]
