FROM scratch
ADD controller-linux /controller
# Mount as configmap, re-read on every change
WORKDIR /script
CMD ["/controller", "-v", "4", "--logtostderr"]
