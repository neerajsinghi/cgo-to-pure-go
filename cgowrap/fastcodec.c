//go:build cgo

/*
 * fastcodec.c: implementation of fastcodec.h. See the header for the API
 * contract. Written in portable C99; all multi-byte loads are assembled
 * byte-by-byte so the result does not depend on host endianness.
 */
#include "fastcodec.h"

#include <string.h>

#define FC_CRC32C_POLY 0x82F63B78u

static uint32_t fc_table[8][256];

void fc_init(void) {
    for (uint32_t i = 0; i < 256; i++) {
        uint32_t c = i;
        for (int k = 0; k < 8; k++) {
            c = (c & 1u) ? (c >> 1) ^ FC_CRC32C_POLY : (c >> 1);
        }
        fc_table[0][i] = c;
    }
    for (uint32_t i = 0; i < 256; i++) {
        uint32_t c = fc_table[0][i];
        for (int j = 1; j < 8; j++) {
            c = fc_table[0][c & 0xffu] ^ (c >> 8);
            fc_table[j][i] = c;
        }
    }
}

/* load32le reads 4 bytes as a little-endian uint32 on any host. The casts
 * matter: p[3] << 24 on a promoted int would be undefined behaviour. */
static inline uint32_t load32le(const uint8_t *p) {
    return (uint32_t)p[0] | ((uint32_t)p[1] << 8) | ((uint32_t)p[2] << 16) |
           ((uint32_t)p[3] << 24);
}

uint32_t fc_crc32c(uint32_t crc, const uint8_t *buf, size_t len) {
    const uint8_t *p = buf;
    crc = ~crc;
    while (len >= 8) {
        uint32_t lo = load32le(p) ^ crc;
        uint32_t hi = load32le(p + 4);
        crc = fc_table[7][lo & 0xffu] ^ fc_table[6][(lo >> 8) & 0xffu] ^
              fc_table[5][(lo >> 16) & 0xffu] ^ fc_table[4][lo >> 24] ^
              fc_table[3][hi & 0xffu] ^ fc_table[2][(hi >> 8) & 0xffu] ^
              fc_table[1][(hi >> 16) & 0xffu] ^ fc_table[0][hi >> 24];
        p += 8;
        len -= 8;
    }
    while (len--) {
        crc = fc_table[0][(crc ^ *p++) & 0xffu] ^ (crc >> 8);
    }
    return ~crc;
}

size_t fc_uvarint_put(uint8_t *dst, uint64_t v) {
    size_t i = 0;
    while (v >= 0x80u) {
        dst[i++] = (uint8_t)(v | 0x80u);
        v >>= 7;
    }
    dst[i++] = (uint8_t)v;
    return i;
}

int fc_uvarint_get(const uint8_t *src, size_t len, uint64_t *out, size_t *used) {
    uint64_t x = 0;
    unsigned s = 0;
    for (size_t i = 0; i < len; i++) {
        if (i == FC_MAX_VARINT_LEN64) {
            return FC_ERR_OVERFLOW;
        }
        uint8_t b = src[i];
        if (b < 0x80u) {
            if (i == FC_MAX_VARINT_LEN64 - 1 && b > 1) {
                return FC_ERR_OVERFLOW;
            }
            *out = x | ((uint64_t)b << s);
            *used = i + 1;
            return FC_OK;
        }
        x |= (uint64_t)(b & 0x7fu) << s;
        s += 7;
    }
    return FC_ERR_SHORT;
}

int fc_rle_encode(uint8_t *dst, size_t cap, const uint8_t *src, size_t len,
                  size_t *written) {
    size_t i = 0, o = 0;
    while (i < len) {
        uint8_t v = src[i];
        size_t run = 1;
        while (i + run < len && src[i + run] == v && run < 255) {
            run++;
        }
        if (cap - o < 2) {
            return FC_ERR_DST;
        }
        dst[o++] = (uint8_t)run;
        dst[o++] = v;
        i += run;
    }
    *written = o;
    return FC_OK;
}

int fc_rle_decoded_len(const uint8_t *src, size_t len, size_t *out) {
    if (len % 2 != 0) {
        return FC_ERR_CORRUPT;
    }
    size_t n = 0;
    for (size_t i = 0; i < len; i += 2) {
        if (src[i] == 0) {
            return FC_ERR_CORRUPT;
        }
        n += src[i];
    }
    *out = n;
    return FC_OK;
}

int fc_rle_decode(uint8_t *dst, size_t cap, const uint8_t *src, size_t len,
                  size_t *written) {
    if (len % 2 != 0) {
        return FC_ERR_CORRUPT;
    }
    size_t o = 0;
    for (size_t i = 0; i < len; i += 2) {
        size_t count = src[i];
        if (count == 0) {
            return FC_ERR_CORRUPT;
        }
        if (cap - o < count) {
            return FC_ERR_DST;
        }
        memset(dst + o, src[i + 1], count);
        o += count;
    }
    *written = o;
    return FC_OK;
}
