FROM nginx:1.27

RUN apt-get update \
	&& apt-get install -y --no-install-recommends openssh-server \
	&& rm -rf /var/lib/apt/lists/* \
	&& ssh-keygen -A \
	&& echo 'root:aegisops123' | chpasswd \
	&& sed -i 's/^#\?PermitRootLogin .*/PermitRootLogin yes/' /etc/ssh/sshd_config \
	&& sed -i 's/^#\?PasswordAuthentication .*/PasswordAuthentication yes/' /etc/ssh/sshd_config \
	&& mkdir -p /run/sshd

EXPOSE 22 80

CMD ["/bin/sh", "-c", "/usr/sbin/sshd && nginx -g 'daemon off;'"]
