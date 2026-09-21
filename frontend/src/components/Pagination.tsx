import { Button } from "./Button";

export function Pagination({
  page,
  totalPages,
  total,
  onPageChange,
}: {
  page: number;
  totalPages: number;
  total?: number;
  onPageChange(page: number): void;
}) {
  if (totalPages <= 1)
    return total === undefined ? null : (
      <p className="result-count">{total} results</p>
    );
  return (
    <nav className="pagination" aria-label="Pagination">
      <span>
        Page {page} of {totalPages}{" "}
        {total === undefined ? "" : `· ${total} results`}
      </span>
      <div>
        <Button
          variant="secondary"
          size="compact"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          Previous
        </Button>
        <Button
          variant="secondary"
          size="compact"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          Next
        </Button>
      </div>
    </nav>
  );
}
