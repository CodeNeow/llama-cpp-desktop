/**
 * Portrait tablet band (768..1099px) card-composition helpers for the Home
 * System Info tab. Pure predicates only — no Vue reactivity, no backend —
 * so the composition rules stay unit-testable.
 */

/**
 * Whether the portrait tablet band should pair the resident-model card with
 * the system summary card in one two-column row below the storage island
 * (both are compact summary strips, so sharing a row fills the band's dead
 * bottom area without crowding either card). Exactly one resident card is
 * required: with zero residents there is nothing to pair with (first-use /
 * stopped service keeps the single-column stack unchanged), and with two or
 * more each card's chip row needs the full width. The caller passes the
 * resident-model count.
 */
export function pairsResidentWithSystemCard(residentCount: number): boolean {
  return residentCount === 1
}
