
This package serves two purposes.

1. It's the primary Helm chart for folks wanting to run their own Streamplace. So it specifies all
   the other common packages in its `requirement.yaml`.

1. It'll allow developers/Linux users to install the desktop app with `npm install -g streamplace`.

This package shouldn't be used by people that want to use the Streamplace API -- you're looking
for `sp-client` for that.
