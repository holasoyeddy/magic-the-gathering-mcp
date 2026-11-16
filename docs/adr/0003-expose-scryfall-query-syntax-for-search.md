# Expose Scryfall query syntax for search

Card search will accept Scryfall's native query syntax instead of defining a smaller server-specific filter language. This deliberately couples the search contract to the chosen upstream, but gives every MCP client the full, documented search language immediately and avoids maintaining an incomplete parallel query model.
