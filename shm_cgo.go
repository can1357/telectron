//go:build darwin && cgo

package main

/*
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <string.h>
#include <stdlib.h>

// shm_open is variadic (mode is optional), which cgo cannot call directly, so
// wrap it in a fixed-arity shim.
static int te_shm_open(const char* name, int oflag, unsigned short mode) {
    return shm_open(name, oflag, (mode_t)mode);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// shmAvailable reports that the shared-memory medium is usable in this build.
const shmAvailable = true

// shmWrite creates a fresh POSIX shared memory object `name` holding data, then
// closes it (leaving the object for the terminal to open, read, and unlink).
//
// It first unlinks any object currently bound to the name and creates the new
// one with O_EXCL. macOS rejects ftruncate on a shm object that already has a
// size, so reusing a slot name requires a brand-new object each time. The
// previous object (if any) was already consumed by the terminal — the caller's
// per-pool barrier guarantees this before a slot is reused — so unlinking the
// name only drops the stale binding; any reader still holding it keeps its copy.
func shmWrite(name string, data []byte) error {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	C.shm_unlink(cname) // drop any prior object bound to this name
	fd := C.te_shm_open(cname, C.O_CREAT|C.O_RDWR|C.O_EXCL, 0o600)
	if fd < 0 {
		return fmt.Errorf("shm_open %s failed", name)
	}
	defer C.close(fd)

	if C.ftruncate(fd, C.off_t(len(data))) != 0 {
		return fmt.Errorf("ftruncate %s (%d bytes) failed", name, len(data))
	}
	if len(data) == 0 {
		return nil
	}

	ptr := C.mmap(nil, C.size_t(len(data)), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, fd, 0)
	if uintptr(ptr) == ^uintptr(0) { // MAP_FAILED == (void*)-1
		return fmt.Errorf("mmap %s failed", name)
	}
	defer C.munmap(ptr, C.size_t(len(data)))
	C.memcpy(ptr, unsafe.Pointer(&data[0]), C.size_t(len(data)))
	return nil
}

// shmUnlink removes a shared memory object by name, ignoring errors (it may
// already have been unlinked by the terminal).
func shmUnlink(name string) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.shm_unlink(cname)
}

// shmRead reads n bytes from the named shared memory object. Used by tests to
// verify what a client wrote is what a reader (the terminal) would see.
func shmRead(name string, n int) ([]byte, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	fd := C.te_shm_open(cname, C.O_RDONLY, 0)
	if fd < 0 {
		return nil, fmt.Errorf("shm_open %s (read) failed", name)
	}
	defer C.close(fd)
	if n == 0 {
		return nil, nil
	}
	ptr := C.mmap(nil, C.size_t(n), C.PROT_READ, C.MAP_SHARED, fd, 0)
	if uintptr(ptr) == ^uintptr(0) {
		return nil, fmt.Errorf("mmap %s (read) failed", name)
	}
	defer C.munmap(ptr, C.size_t(n))
	return C.GoBytes(ptr, C.int(n)), nil
}
