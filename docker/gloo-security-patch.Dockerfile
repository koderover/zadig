ARG BASE_IMAGE
FROM ${BASE_IMAGE}

USER 0
ARG APK_PACKAGES
RUN apk add --upgrade --no-cache ${APK_PACKAGES:?APK_PACKAGES must list pinned packages}

USER 10101
