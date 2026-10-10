#include "http.h"

#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
typedef SOCKET socket_t;
#define INVALID_SOCK INVALID_SOCKET
#define close_socket closesocket
#else
#include <errno.h>
#include <fcntl.h>
#include <netdb.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <poll.h>
#include <sys/socket.h>
#include <unistd.h>
typedef int socket_t;
#define INVALID_SOCK (-1)
#define close_socket close
#endif

#ifdef MSG_NOSIGNAL
#define SEND_FLAGS MSG_NOSIGNAL
#else
#define SEND_FLAGS 0
#endif

#define CONNECT_TIMEOUT_SEC 5
#define IO_TIMEOUT_SEC 10
#define MAX_RESPONSE_SIZE (1024 * 1024)

void http_init(void)
{
#ifdef _WIN32
	WSADATA wsa;
	WSAStartup(MAKEWORD(2, 2), &wsa);
#endif
}

void http_free(void)
{
#ifdef _WIN32
	WSACleanup();
#endif
}

bool http_parse_url(const char *url, struct http_url *out, struct dstr *err)
{
	static const char scheme[] = "http://";
	memset(out, 0, sizeof(*out));

	if (astrcmpi_n(url, scheme, sizeof(scheme) - 1) != 0) {
		dstr_printf(err, "Unsupported URL '%s': only http:// URLs are supported", url);
		return false;
	}

	const char *host = url + sizeof(scheme) - 1;
	const char *path = strchr(host, '/');
	const char *host_end = path ? path : host + strlen(host);
	const char *port = NULL;

	if (*host == '[') {
		const char *close = memchr(host, ']', host_end - host);
		if (close) {
			dstr_ncopy(&out->host, host + 1, close - host - 1);
			if (close[1] == ':')
				port = close + 2;
		}
	} else {
		const char *colon = memchr(host, ':', host_end - host);
		dstr_ncopy(&out->host, host, (colon ? colon : host_end) - host);
		if (colon)
			port = colon + 1;
	}

	if (port && port < host_end)
		dstr_ncopy(&out->port, port, host_end - port);
	else
		dstr_copy(&out->port, "80");
	dstr_copy(&out->path, path ? path : "/");

	if (dstr_is_empty(&out->host)) {
		dstr_printf(err, "Invalid URL '%s'", url);
		http_url_free(out);
		return false;
	}
	return true;
}

void http_url_free(struct http_url *url)
{
	dstr_free(&url->host);
	dstr_free(&url->port);
	dstr_free(&url->path);
}

static void set_nonblocking(socket_t s, bool nonblocking)
{
#ifdef _WIN32
	u_long mode = nonblocking;
	ioctlsocket(s, FIONBIO, &mode);
#else
	int flags = fcntl(s, F_GETFL, 0);
	fcntl(s, F_SETFL, nonblocking ? flags | O_NONBLOCK : flags & ~O_NONBLOCK);
#endif
}

static bool connect_with_timeout(socket_t s, const struct sockaddr *addr, socklen_t len)
{
	set_nonblocking(s, true);
	if (connect(s, addr, len) != 0) {
#ifdef _WIN32
		if (WSAGetLastError() != WSAEWOULDBLOCK)
			return false;
		/* Winsock's fd_set is a list of sockets, so any socket value fits. */
		fd_set wfds, efds;
		FD_ZERO(&wfds);
		FD_ZERO(&efds);
		FD_SET(s, &wfds);
		FD_SET(s, &efds);
		struct timeval tv = {CONNECT_TIMEOUT_SEC, 0};
		if (select(0, NULL, &wfds, &efds, &tv) <= 0)
			return false;
#else
		if (errno != EINPROGRESS)
			return false;
		/* poll, not select: descriptors past FD_SETSIZE are common in OBS. */
		struct pollfd pfd = {.fd = s, .events = POLLOUT};
		if (poll(&pfd, 1, CONNECT_TIMEOUT_SEC * 1000) <= 0)
			return false;
#endif

		int so_error = 0;
		socklen_t so_len = sizeof(so_error);
		if (getsockopt(s, SOL_SOCKET, SO_ERROR, (char *)&so_error, &so_len) != 0 || so_error != 0)
			return false;
	}
	set_nonblocking(s, false);
	return true;
}

static void configure_socket(socket_t s)
{
#ifdef _WIN32
	DWORD timeout = IO_TIMEOUT_SEC * 1000;
#else
	struct timeval timeout = {IO_TIMEOUT_SEC, 0};
#endif
	setsockopt(s, SOL_SOCKET, SO_RCVTIMEO, (const char *)&timeout, sizeof(timeout));
	setsockopt(s, SOL_SOCKET, SO_SNDTIMEO, (const char *)&timeout, sizeof(timeout));

	int one = 1;
	setsockopt(s, IPPROTO_TCP, TCP_NODELAY, (const char *)&one, sizeof(one));
#ifdef SO_NOSIGPIPE
	setsockopt(s, SOL_SOCKET, SO_NOSIGPIPE, &one, sizeof(one));
#endif
}

static socket_t open_socket(const struct http_url *url, struct dstr *err)
{
	struct addrinfo hints = {0};
	hints.ai_family = AF_UNSPEC;
	hints.ai_socktype = SOCK_STREAM;

	struct addrinfo *res = NULL;
	if (getaddrinfo(url->host.array, url->port.array, &hints, &res) != 0) {
		dstr_printf(err, "Could not resolve '%s'", url->host.array);
		return INVALID_SOCK;
	}

	socket_t s = INVALID_SOCK;
	for (struct addrinfo *ai = res; ai; ai = ai->ai_next) {
		s = socket(ai->ai_family, ai->ai_socktype, ai->ai_protocol);
		if (s == INVALID_SOCK)
			continue;
		if (connect_with_timeout(s, ai->ai_addr, (socklen_t)ai->ai_addrlen))
			break;
		close_socket(s);
		s = INVALID_SOCK;
	}
	freeaddrinfo(res);

	if (s == INVALID_SOCK)
		dstr_printf(err, "Could not connect to %s:%s", url->host.array, url->port.array);
	else
		configure_socket(s);
	return s;
}

static bool send_all(socket_t s, const void *data, size_t size)
{
	const char *p = data;
	while (size) {
		int n = send(s, p, size > INT_MAX ? INT_MAX : (int)size, SEND_FLAGS);
		if (n <= 0)
			return false;
		p += n;
		size -= (size_t)n;
	}
	return true;
}

static bool send_request_head(socket_t s, const struct http_url *url, const char *content_type,
			      const char *extra_headers)
{
	struct dstr head = {0};
	bool ipv6 = strchr(url->host.array, ':') != NULL;
	dstr_printf(&head,
		    "POST %s HTTP/1.1\r\n"
		    "Host: %s%s%s:%s\r\n"
		    "User-Agent: obs-streamplace\r\n"
		    "Content-Type: %s\r\n"
		    "%s\r\n",
		    url->path.array, ipv6 ? "[" : "", url->host.array, ipv6 ? "]" : "", url->port.array, content_type,
		    extra_headers);
	bool ok = send_all(s, head.array, head.len);
	dstr_free(&head);
	return ok;
}

static bool decode_chunked(const char *p, const char *end, struct dstr *out)
{
	while (p < end) {
		char *size_end;
		unsigned long size = strtoul(p, &size_end, 16);
		const char *data = strstr(size_end, "\r\n");
		if (!data)
			return false;
		data += 2;
		if (size == 0)
			return true;
		if ((size_t)(end - data) < size)
			return false;
		dstr_ncat(out, data, size);
		p = data + size + 2;
	}
	return false;
}

/* Reads until the server closes the connection, then splits the response. */
static bool read_response(socket_t s, long *status, struct dstr *body)
{
	struct dstr raw = {0};
	char buf[4096];
	int n;
	while (raw.len < MAX_RESPONSE_SIZE && (n = recv(s, buf, sizeof(buf), 0)) > 0)
		dstr_ncat(&raw, buf, n);

	bool ok = false;
	const char *header_end = raw.array ? strstr(raw.array, "\r\n\r\n") : NULL;
	if (header_end && sscanf(raw.array, "HTTP/%*d.%*d %ld", status) == 1) {
		struct dstr headers = {0};
		dstr_ncopy(&headers, raw.array, header_end - raw.array);
		const char *start = header_end + 4;
		const char *end = raw.array + raw.len;

		dstr_free(body);
		if (astrstri(headers.array, "transfer-encoding: chunked")) {
			ok = decode_chunked(start, end, body);
		} else {
			dstr_ncopy(body, start, end - start);
			ok = true;
		}
		dstr_free(&headers);
	}

	dstr_free(&raw);
	return ok;
}

bool http_post_json(const char *url_str, const char *body, long *status, struct dstr *response, struct dstr *err)
{
	struct http_url url;
	if (!http_parse_url(url_str, &url, err))
		return false;

	socket_t s = open_socket(&url, err);
	bool ok = false;
	if (s != INVALID_SOCK) {
		struct dstr headers = {0};
		dstr_printf(&headers, "Content-Length: %lu\r\nConnection: close\r\n", (unsigned long)strlen(body));
		ok = send_request_head(s, &url, "application/json", headers.array) && send_all(s, body, strlen(body)) &&
		     read_response(s, status, response);
		if (!ok)
			dstr_printf(err, "Request to %s failed", url_str);
		dstr_free(&headers);
		close_socket(s);
	}

	http_url_free(&url);
	return ok;
}

bool http_begin_chunked_post(struct http_conn *conn, const char *url_str, const char *content_type, struct dstr *err)
{
	struct http_url url;
	if (!http_parse_url(url_str, &url, err))
		return false;

	socket_t s = open_socket(&url, err);
	bool ok = s != INVALID_SOCK &&
		  send_request_head(s, &url, content_type, "Transfer-Encoding: chunked\r\nConnection: close\r\n");
	if (ok) {
		conn->sock = (intptr_t)s;
	} else if (s != INVALID_SOCK) {
		dstr_printf(err, "Could not send request to %s", url_str);
		close_socket(s);
	}

	http_url_free(&url);
	return ok;
}

bool http_write_chunk(struct http_conn *conn, const uint8_t *data, size_t size)
{
	char header[32];
	int len = snprintf(header, sizeof(header), "%lx\r\n", (unsigned long)size);
	socket_t s = (socket_t)conn->sock;
	return send_all(s, header, (size_t)len) && send_all(s, data, size) && send_all(s, "\r\n", 2);
}

long http_end_chunked_post(struct http_conn *conn)
{
	socket_t s = (socket_t)conn->sock;
	long status = 0;
	struct dstr body = {0};
	if (send_all(s, "0\r\n\r\n", 5))
		read_response(s, &status, &body);
	dstr_free(&body);
	return status;
}

void http_close(struct http_conn *conn)
{
	close_socket((socket_t)conn->sock);
	conn->sock = (intptr_t)INVALID_SOCK;
}
