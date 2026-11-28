#ifndef STATION_SLATE_H
#define STATION_SLATE_H

#include <stddef.h>
#include <stdint.h>

typedef struct station_slate_engine station_slate_engine;
typedef struct station_slate_db station_slate_db;
typedef struct station_slate_bytes station_slate_bytes;

typedef enum {
    STATION_SLATE_OK,
    STATION_SLATE_NOT_FOUND,
    STATION_SLATE_FENCED,
    STATION_SLATE_INVALID,
    STATION_SLATE_FAILED,
} station_slate_status;

typedef struct {
    const uint8_t *data;
    size_t len;
} station_slate_slice;

typedef struct {
    station_slate_slice data;
    station_slate_bytes *owner;
} station_slate_value;

/* Nonempty input slices must be readable, immutable single allocations for the call.
 * Outputs must be writable, aligned and disjoint from input and handle storage. */
station_slate_status station_slate_engine_new(station_slate_slice endpoint,
    station_slate_slice bucket, station_slate_slice access_key,
    station_slate_slice secret_key, station_slate_slice region,
    station_slate_engine **out);
station_slate_status station_slate_epoch(const station_slate_engine *engine,
    station_slate_slice path, uint64_t *out);
station_slate_status station_slate_open(const station_slate_engine *engine,
    station_slate_slice path, uint64_t expected_epoch, station_slate_db **out);
station_slate_status station_slate_get(const station_slate_db *db,
    station_slate_slice key, station_slate_value *out);
station_slate_status station_slate_put(const station_slate_db *db,
    station_slate_slice key, station_slate_slice value);

/* Close/free consumes exactly one live handle; it cannot race other uses of it.
 * A successful get borrows its data from owner until value_free. */
station_slate_status station_slate_close(station_slate_db *db);
void station_slate_engine_free(station_slate_engine *engine);
void station_slate_value_free(station_slate_bytes *owner);

#endif
