#pragma once

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include <util/dstr.h>

/* A deliberately small HTTP/1.1 client over plain TCP: the plugin talks to a
 * Streamplace node on the same machine, so there is no TLS. */

struct http_url {
	struct dstr host;
	struct dstr port;
	struct dstr path;
};

struct http_conn {
	intptr_t sock;
};

void http_init(void);
void http_free(void);

bool http_parse_url(const char *url, struct http_url *out, struct dstr *err);
void http_url_free(struct http_url *url);

/* POSTs a JSON body and returns the response body. */
bool http_post_json(const char *url, const char *body, long *status, struct dstr *response, struct dstr *err);

/* Opens a POST with a chunked request body. */
bool http_begin_chunked_post(struct http_conn *conn, const char *url, const char *content_type, struct dstr *err);
bool http_write_chunk(struct http_conn *conn, const uint8_t *data, size_t size);
/* Terminates the body and returns the response status, or 0 if none arrived. */
long http_end_chunked_post(struct http_conn *conn);
void http_close(struct http_conn *conn);
