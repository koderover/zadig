#ubuntu-xenial.Dockerfile

COPY docker/dist/reaper /usr/local/bin

ENTRYPOINT ["reaper"]
