// Package dc is the bot's internal Discord facade.
//
// Every type here is library-neutral: it mirrors the shape of the Discord API
// rather than the shape of any Go client. Handlers, command definitions and
// message payloads are expressed in these types, and this package alone
// converts them to whatever client is linked in.
//
// It converts to github.com/disgoorg/disgo. Swapping the client out replaced
// the internals of this package only; call sites in the rest of the bot were
// left untouched.
//
// The facade grows on demand. Only the pieces a migrated package needs are
// modelled, so an absent type means "not migrated yet", not "unsupported".
package dc
