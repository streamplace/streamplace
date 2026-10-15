#include <string.h>
#include "streamplace.h"

/* GLib and GObject install their initializers as static constructors, and so
 * does the Go runtime: it starts its thread from .init_array, and the gotk4
 * bindings register GLib types when that thread runs package inits. The linker
 * only pulls an archive member once something references a symbol from it, and
 * it reaches the Go archive before GLib's, so those package inits can run
 * against a type system that GLib has not filled in yet ("cannot derive type
 * 'GSocketFamily' from invalid parent type").
 *
 * The two initializers share an object file with the two symbols below, so
 * referring to them from this first-linked translation unit pulls those
 * objects in ahead of the Go archive, which orders both constructors ahead of
 * the Go runtime's in .init_array. Neither symbol is called: g_type_init()
 * asserts that the type system is already initialized, and here it is not.
 */
extern void glib_init(void);
extern void g_type_init(void);
void (*const streamplace_static_init_refs[])(void) = {
  glib_init,
  g_type_init,
};

int main(int argc, char *argv[])
{
  StreamplaceMain();
}
