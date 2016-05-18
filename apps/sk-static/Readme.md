sk-static
=========

This is Stream Kitchen's container for hosting static files. Like all the SK containers, it's
based on Ubuntu Xenial.

The only feature that this has above a generic nginx container is that we do a runtime mustache
compilation, whereby any files at `/web/**/*.mustache` will be compiled using the current
environment variables, output in the same file without the `.mustache` extension, and have their
original template removed. This is a useful and lightweight way to configure containers at runtime.
