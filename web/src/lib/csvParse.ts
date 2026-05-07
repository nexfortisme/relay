export type ParsedCsv = {
  rows: string[][]
  truncated: boolean
}

/** RFC 4180–style CSV parse (comma delimiter, quoted fields, escaped quotes). */
export function parseCsvRows(input: string, maxRows = 5000): ParsedCsv {
  const s = input.replace(/^\uFEFF/, "")
  const rows: string[][] = []
  if (maxRows <= 0) {
    return { rows: [], truncated: false }
  }
  let row: string[] = []
  let field = ""
  let i = 0
  let inQuotes = false
  let truncated = false
  const len = s.length

  const pushField = () => {
    row.push(field)
    field = ""
  }

  const pushRow = () => {
    pushField()
    rows.push(row)
    row = []
    if (rows.length >= maxRows) {
      truncated = true
    }
  }

  while (i < len && !truncated) {
    const c = s[i]!
    if (inQuotes) {
      if (c === '"') {
        if (i + 1 < len && s[i + 1] === '"') {
          field += '"'
          i += 2
        } else {
          inQuotes = false
          i += 1
        }
      } else {
        field += c
        i += 1
      }
    } else if (c === '"') {
      inQuotes = true
      i += 1
    } else if (c === ",") {
      pushField()
      i += 1
    } else if (c === "\r") {
      i += 1
      if (i < len && s[i] === "\n") {
        i += 1
      }
      pushRow()
    } else if (c === "\n") {
      i += 1
      pushRow()
    } else {
      field += c
      i += 1
    }
  }

  if (!truncated && (field.length > 0 || row.length > 0)) {
    pushField()
    if (row.length > 0) {
      rows.push(row)
      if (rows.length >= maxRows) {
        truncated = true
      }
    }
  }

  return { rows, truncated }
}

/** Excel-style column label: A, B, …, Z, AA, AB, … */
export function columnLetter(indexZeroBased: number): string {
  let n = indexZeroBased + 1
  let label = ""
  while (n > 0) {
    const rem = (n - 1) % 26
    label = String.fromCharCode(65 + rem) + label
    n = Math.floor((n - 1) / 26)
  }
  return label
}
