import styled from 'styled-components'

export const AnalysisSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 16px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fafafa;
`

export const SectionTitle = styled.h2`
  margin: 0;
  color: #111;
  font-size: 18px;
`

export const Message = styled.p`
  color: #666;
  font-size: 16px;
`

type AnalysisPipelineState = 'completed' | 'active' | 'waiting' | 'pending' | 'error'

const pipelineStateColor = (state: AnalysisPipelineState): string => {
  if (state === 'completed') return '#2a8a2a'
  if (state === 'active') return '#33408f'
  if (state === 'waiting') return '#8a5a00'
  if (state === 'error') return '#b42318'
  return '#888'
}

export const AnalysisStatusTitle = styled.h3`
  margin: 4px 0 0;
  color: #111;
  font-size: 18px;
`

export const AnalysisPipeline = styled.div`
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  margin: 4px 0;

  @media (max-width: 639px) {
    grid-template-columns: 1fr;
    gap: 4px;
  }
`

export const AnalysisPipelineItem = styled.div<{ $state: AnalysisPipelineState }>`
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid ${({ $state }) => pipelineStateColor($state)};
  border-radius: 6px;
  background: ${({ $state }) => ($state === 'pending' ? '#fff' : `${pipelineStateColor($state)}12`)};
`

export const AnalysisPipelineMarker = styled.span<{ $state: AnalysisPipelineState }>`
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: ${({ $state }) => pipelineStateColor($state)};
  color: #fff;
  font-size: 13px;
  font-weight: 700;
`

export const AnalysisPipelineInfo = styled.div`
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
`

export const AnalysisPipelineName = styled.strong`
  color: #222;
  font-size: 14px;
`

export const AnalysisPipelineStatus = styled.span`
  color: #666;
  font-size: 12px;
`

export const AnalysisPipelineArrow = styled.span`
  color: #aaa;
  font-size: 20px;
  line-height: 1;
  text-align: center;

  @media (max-width: 639px) {
    transform: rotate(90deg);
  }
`

export const AudienceList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
`

export const AudienceItem = styled.li`
  border: 1px solid #e2e2e2;
  border-radius: 6px;
  background: #fff;
`

export const ReviewItem = styled.label`
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #e2e2e2;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
`

export const ReviewCheckbox = styled.input`
  margin-top: 3px;
`

export const ReviewForm = styled.div`
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid #d8d8d8;
  border-radius: 6px;
  background: #fff;
`

export const ReviewHeading = styled.h3`
  margin: 0;
  color: #222;
  font-size: 16px;
`

export const AudienceDetails = styled.details`
  width: 100%;

  &[open] > summary::before {
    transform: rotate(90deg);
  }
`

export const AudienceSummary = styled.summary`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 600;
  user-select: none;

  &::before {
    content: '›';
    color: #666;
    font-size: 22px;
    line-height: 1;
    transition: transform 120ms ease;
  }

  &:hover {
    background: #f0f0f0;
  }

  &:focus-visible {
    outline: 2px solid #333;
    outline-offset: 2px;
  }

  &::-webkit-details-marker {
    display: none;
  }
`

export const SummaryLabel = styled.span`
  min-width: 0;
`

export const ViewToggle = styled.span`
  display: inline-flex;
  flex: 0 0 auto;
  margin-left: auto;
  padding: 2px;
  border: 1px solid #d8d8d8;
  border-radius: 5px;
  background: #f7f7f7;
`

export const ViewToggleButton = styled.button`
  padding: 4px 8px;
  border: 0;
  border-radius: 3px;
  background: transparent;
  color: #777;
  font-size: 11px;
  cursor: pointer;

  &[aria-pressed='true'] {
    background: #fff;
    box-shadow: 0 1px 2px rgb(0 0 0 / 12%);
    color: #222;
    font-weight: 700;
  }

  &:focus-visible {
    outline: 2px solid #333;
    outline-offset: 1px;
  }
`

export const JsonPreview = styled.pre`
  max-height: 680px;
  margin: 0;
  padding: 16px;
  overflow: auto;
  border-top: 1px solid #e2e2e2;
  background: #1f2329;
  color: #e6edf3;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre;
  tab-size: 2;
`

export const AudienceInfo = styled.div`
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
`

export const AudienceDescription = styled.span`
  color: #333;
  font-size: 14px;
  font-weight: 600;
  word-break: break-all;
`

export const HypothesisStatement = styled.span`
  display: block;
  color: #555;
  font-size: 16px;
  line-height: 1.45;
`

export const AudienceMeta = styled.span`
  color: #888;
  font-size: 12px;
`

export const ResultsTitle = styled.h3`
  margin: 4px 0 0;
  color: #111;
  font-size: 18px;
`

export const Field = styled.label`
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: #333;
  font-size: 14px;
`

export const Input = styled.input`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
`

export const TextArea = styled.textarea`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
  resize: vertical;
`

export const ButtonGroup = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
`

export const Button = styled.button<{ $success?: boolean }>`
  padding: 10px 16px;
  border: 1px solid ${({ $success }) => ($success ? '#2a8a2a' : '#ccc')};
  border-radius: 6px;
  background: ${({ $success }) => ($success ? '#2a8a2a' : '#fff')};
  color: ${({ $success }) => ($success ? '#fff' : '#111')};
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: ${({ $success }) => ($success ? '#237023' : '#f5f5f5')};
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const ErrorMessage = styled.p`
  color: #c00;
  font-size: 16px;
`
