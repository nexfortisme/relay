import { describe, expect, it } from "vitest";
import { columnLetter, parseCsvRows } from "../lib/csvParse";

describe("csvParse", () => {
  it("parses simple comma-separated rows", () => {
    expect(parseCsvRows("a,b,c\n1,2,3")).toEqual({
      rows: [
        ["a", "b", "c"],
        ["1", "2", "3"],
      ],
      truncated: false,
    });
  });

  it("supports quoted fields and escaped quotes", () => {
    expect(parseCsvRows('"hello, world",x\n"""quoted""",y')).toEqual({
      rows: [
        ["hello, world", "x"],
        ['"quoted"', "y"],
      ],
      truncated: false,
    });
  });

  it("strips a UTF-8 BOM", () => {
    expect(parseCsvRows("\uFEFFone,two")).toEqual({
      rows: [["one", "two"]],
      truncated: false,
    });
  });

  it("handles CRLF and a final line without newline", () => {
    expect(parseCsvRows("p,q\r\nr,s")).toEqual({
      rows: [
        ["p", "q"],
        ["r", "s"],
      ],
      truncated: false,
    });
  });

  it("truncates when row budget is exceeded", () => {
    const { rows, truncated } = parseCsvRows("a\nb\nc", 2);
    expect(rows).toEqual([["a"], ["b"]]);
    expect(truncated).toBe(true);
  });

  it("maps zero-based column indexes to Excel-style letters", () => {
    expect(columnLetter(0)).toBe("A");
    expect(columnLetter(25)).toBe("Z");
    expect(columnLetter(26)).toBe("AA");
    expect(columnLetter(701)).toBe("ZZ");
  });
});
