# AWS Emulator のイメージ(公式は既製イメージを配らず、この形の Dockerfile を案内している。
# AWS Emulator User Guide「Getting started with Docker」)。版は固定し、上げるときは scripts/tf-aws-emulator.sh の
# AWS_EMULATOR_VERSION と一緒に変える。Linux なら Docker を使わず Debian/RPM パッケージを直接入れてもよい(README)。
FROM public.ecr.aws/amazonlinux/amazonlinux:2023-minimal
ARG AWS_EMULATOR_VERSION
ARG AWS_EMULATOR_ARCH
ADD https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/aws-emulator-${AWS_EMULATOR_VERSION}-1-${AWS_EMULATOR_ARCH}.rpm /tmp/aws-emulator.rpm
RUN rpm -i /tmp/aws-emulator.rpm && rm /tmp/aws-emulator.rpm \
    && mkdir -p /home/emulator && chown 1000 /home/emulator
USER 1000
ENV HOME=/home/emulator
EXPOSE 9297
ENTRYPOINT ["aws-emulator", "-b", "0.0.0.0"]
