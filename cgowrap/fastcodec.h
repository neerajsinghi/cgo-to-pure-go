//go:build cgo

/*
 * fastcodec: a tiny, self-contained C library used as the "legacy native
 * dependency" in this case study. It provides three well-specified codecs:
 *
 *   - CRC-32C (Castagnoli, reflected polynomial 0x82F63B78), slicing-by-8.
 *   - Unsigned LEB128 varints (the same wire format as Go's encoding/binary
 *     Uvarint and Protocol Buffers).
 *   - A byte-oriented run-length encoding: a sequence of (count, value) byte
 *     pairs where 1 <= count <= 255.
 *
 * Errors are reported through return codes (never errno), and all buffers are
 * owned by the caller: the library never allocates or frees memory.
 */
#ifndef FASTCODEC_H
#define FASTCODEC_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Status codes returned by the fallible functions. */
#define FC_OK 0
#define FC_ERR_SHORT (-1)    /* input ended in the middle of a value */
#define FC_ERR_OVERFLOW (-2) /* varint does not fit in 64 bits */
#define FC_ERR_CORRUPT (-3)  /* malformed RLE stream */
#define FC_ERR_DST (-4)      /* destination buffer too small */

/* Maximum encoded length of a 64-bit varint. */
#define FC_MAX_VARINT_LEN64 10

/*
 * fc_init builds the CRC lookup tables. It must be called once before any
 * other function and is not safe to call concurrently with fc_crc32c.
 */
void fc_init(void);

/*
 * fc_crc32c updates a running CRC-32C with len bytes from buf. Start with
 * crc = 0. The result for "123456789" is 0xE3069283.
 */
uint32_t fc_crc32c(uint32_t crc, const uint8_t *buf, size_t len);

/*
 * fc_uvarint_put writes v to dst (which must have room for
 * FC_MAX_VARINT_LEN64 bytes) and returns the number of bytes written.
 */
size_t fc_uvarint_put(uint8_t *dst, uint64_t v);

/*
 * fc_uvarint_get decodes a varint from src[0:len]. On FC_OK it stores the
 * value in *out and the number of bytes consumed in *used.
 */
int fc_uvarint_get(const uint8_t *src, size_t len, uint64_t *out, size_t *used);

/*
 * fc_rle_encode encodes src[0:len] into dst[0:cap]. A buffer of 2*len bytes
 * is always sufficient. On FC_OK *written holds the encoded length.
 */
int fc_rle_encode(uint8_t *dst, size_t cap, const uint8_t *src, size_t len,
                  size_t *written);

/*
 * fc_rle_decoded_len validates an RLE stream and stores its decoded length
 * in *out. It lets callers size the destination before decoding.
 */
int fc_rle_decoded_len(const uint8_t *src, size_t len, size_t *out);

/*
 * fc_rle_decode decodes src[0:len] into dst[0:cap]. On FC_OK *written holds
 * the decoded length.
 */
int fc_rle_decode(uint8_t *dst, size_t cap, const uint8_t *src, size_t len,
                  size_t *written);

#ifdef __cplusplus
}
#endif

#endif /* FASTCODEC_H */
