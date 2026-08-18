// Shared vocabulary for the custom ThemedSelect dropdown: option shape and the
// display-label resolution, kept pure so ThemedSelect.vue stays thin and the
// logic is unit-testable.

export interface SelectOption {
  value: string
  label: string
}

/**
 * Resolve the text shown on the select trigger: the matching option's label,
 * else the raw value when it is non-empty (value not present in options, e.g.
 * a stale persisted choice), else the placeholder.
 */
export function selectDisplayLabel(options: SelectOption[], value: string, placeholder: string): string {
  const match = options.find((o) => o.value === value)
  if (match) return match.label
  if (value) return value
  return placeholder
}
