import { ArrowLeft, ArrowRight } from 'lucide-react'

type Props = {
  page: number
  pageCount: number
  onPageChange: (page: number) => void
}

export default function PaginationControls({ page, pageCount, onPageChange }: Props) {
  if (pageCount <= 1) return null
  return <nav className="pagination-controls" aria-label="List pages">
    <button className="button button-secondary button-small" disabled={page <= 1} onClick={() => onPageChange(page - 1)}><ArrowLeft size={14} /> Previous</button>
    <span>Page <strong>{page}</strong> of <strong>{pageCount}</strong></span>
    <button className="button button-secondary button-small" disabled={page >= pageCount} onClick={() => onPageChange(page + 1)}>Next page <ArrowRight size={14} /></button>
  </nav>
}
