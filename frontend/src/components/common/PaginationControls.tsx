import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import { useIsMobile } from "@/hooks/use-mobile";
import { pageWindow } from "@/lib/pageWindow";

type Props = {
  page: number;
  totalPages: number;
  onPageChange: (p: number) => void;
};

// One link per page used to be rendered unconditionally, so a long history ran
// the control (and the Next arrow at the end of it) off the right edge of a
// phone — see `lib/pageWindow.ts` for the window and why its item count is
// constant. `siblings` is the one thing the layout decides: 0 on a phone, where
// the items are at the 44px tap floor and seven of them is all 390px holds, and
// 1 from 768px up, where they are back to their natural 32px.
export function PaginationControls({ page, totalPages, onPageChange }: Props) {
  const isMobile = useIsMobile();
  const items = pageWindow(page, totalPages, isMobile ? 0 : 1);

  return (
    <Pagination>
      <PaginationContent>
        <PaginationItem>
          <PaginationPrevious
            href="#"
            onClick={(e) => {
              e.preventDefault();
              if (page > 1) onPageChange(page - 1);
            }}
            aria-disabled={page === 1}
            className={page === 1 ? "pointer-events-none opacity-50" : undefined}
            data-testid="pagination-prev"
          />
        </PaginationItem>
        {items.map((item, i) =>
          item === "ellipsis" ? (
            // The index is a safe key here only because the window is positional
            // and holds at most one ellipsis per side.
            <PaginationItem key={`ellipsis-${i}`}>
              <PaginationEllipsis data-testid="pagination-ellipsis" />
            </PaginationItem>
          ) : (
            <PaginationItem key={item}>
              <PaginationLink
                href="#"
                isActive={item === page}
                onClick={(e) => {
                  e.preventDefault();
                  onPageChange(item);
                }}
                data-testid="pagination-page"
              >
                {item}
              </PaginationLink>
            </PaginationItem>
          ),
        )}
        <PaginationItem>
          <PaginationNext
            href="#"
            onClick={(e) => {
              e.preventDefault();
              if (page < totalPages) onPageChange(page + 1);
            }}
            aria-disabled={page === totalPages}
            className={page === totalPages ? "pointer-events-none opacity-50" : undefined}
            data-testid="pagination-next"
          />
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}
