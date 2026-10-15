/* C ABI for the slatedb-store crate. Keep in sync with
 * rust/slatedb-store/src/lib.rs, which is the implementation.
 *
 * Every call returns a status: 0 ok, 1 not found, -1 error. Errors come back as
 * a (pointer, length) pair the caller must release with slatedb_store_free; the
 * same function frees the buffers handed out by the read calls.
 *
 * All buffers are flat, length-prefixed encodings (no structs with pointers):
 *
 *   write batch  [u32 ops][u8 op][u32 key len][u32 value len][key][value]...
 *   scan result  [u32 rows][u32 key len][key][u32 value len][value]...
 */
#ifndef SLATEDB_STORE_H
#define SLATEDB_STORE_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define SLATEDB_OK 0
#define SLATEDB_NOT_FOUND 1
#define SLATEDB_ERROR -1

typedef struct SlateDBStore SlateDBStore;

int32_t slatedb_store_open(const char *store_url, const char *path, uint64_t flush_interval_ms,
                           SlateDBStore **out, uint8_t **err, size_t *err_len);

int32_t slatedb_store_get(const SlateDBStore *handle, const uint8_t *key, size_t key_len,
                          uint8_t **out, size_t *out_len, uint8_t **err, size_t *err_len);

int32_t slatedb_store_write(const SlateDBStore *handle, const uint8_t *payload, size_t payload_len,
                            uint8_t **err, size_t *err_len);

int32_t slatedb_store_scan_prefix(const SlateDBStore *handle, const uint8_t *prefix, size_t prefix_len,
                                  uint8_t **out, size_t *out_len, uint8_t **err, size_t *err_len);

int32_t slatedb_store_delete_prefix(const SlateDBStore *handle, const uint8_t *prefix, size_t prefix_len,
                                    uint8_t **err, size_t *err_len);

int32_t slatedb_store_flush(const SlateDBStore *handle, uint8_t **err, size_t *err_len);

int32_t slatedb_store_close(SlateDBStore *handle, uint8_t **err, size_t *err_len);

void slatedb_store_free(uint8_t *ptr, size_t len);

#ifdef __cplusplus
}
#endif

#endif /* SLATEDB_STORE_H */