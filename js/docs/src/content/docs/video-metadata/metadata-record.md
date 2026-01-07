---
title: "Metadata Record"
sidebar:
  order: 30
---

The `place.stream.metadata.configuration` record is the core structure that
defines how a stream should be presented, used, and distributed. The current
lexicon is a starting point for future iterations.

This record is created by users through the Streamplace frontend and contains
three main components:

1. **Content Warnings** (`place.stream.metadata.configuration.contentWarnings`):
   Users can select content warnings to indicate to node operators and viewers
   what types of warnings have been disclosed. The system supports ten
   predefined warning categories including _violence_, _nudity_, _flashing
   lights_, _language_, _drug use_, _death_, _sexuality_, _suffering_, _fantasy
   violence_, and _personally identifiable information (PII)_. These categories
   are based on the
   [IPTC controlled vocabulary for content warnings](https://cv.iptc.org/newscodes/contentwarning/).
   Each warning provides descriptions to help creators properly categorize their
   content. Streamplace node operators may also configure their nodes to exclude
   certain types of content.
2. **Content Rights** (`place.stream.metadata.configuration.contentRights`):
   This section captures copyright and attribution information, including the
   creator’s name, copyright notice, publication year, license type, and credit
   line. The system supports various pre-defined licensing options from several
   Creative Commons licenses (CC0, CC-BY, CC-BY-SA, CC-BY-NC, CC-BY-NC-SA,
   CC-BY-ND, and CC-BY-NC-ND) to “All Rights Reserved”, as well as the option to
   input custom licensing terms.
3. **Distribution Policy** (`place.stream.metadata.distributionPolicy`): This
   section currently allows creators to specify a `deleteAfter` property, which
   is meant to indicate the time after which the user no longer wants the stream
   to be made available for playback. It also allows you to optionally restrict
   syndication of your livestream to a certain set of broadcasters.

When a user creates or updates their metadata configuration through the
frontend, the record is published to their Personal Data Server (PDS) with the
AT URI pattern `at://[did]/place.stream.metadata.configuration/self`. When a
user goes live, the node extracts this metadata configuration from the local
database and uses it to build C2PA manifests for each stream segment. This
ensures that every piece of the stream carries the creator’s intentions and
requirements for how it should be handled in a tamper-resistant manner. For
detailed schema information,
[see the lexicon reference](/docs/lex-reference/metadata/place-stream-metadata-configuration/).
