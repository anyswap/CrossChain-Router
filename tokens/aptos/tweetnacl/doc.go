/*
tweetnacl-go is a port of Dan Bernstein's "crypto library in a 100 tweets" code to the Go language.
It is implemented as a wrapper around the original code to preserve the design and timing
characteristics of the original implementation.

The Go wrapper has been kept as 'thin' as possible to avoid compromising the careful design
and coding of the original TweetNaCl implementation. However, cryptography being what it is,
the wrapper may have (entirely inadvertently) introduced non-obvious vulnerabilities (for
instance.

http://tweetnacl.cr.yp.to
*/
package tweetnacl
