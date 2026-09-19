import styled from 'styled-components'

export const Page = styled.main`
  max-width: 1120px;
  margin: 40px auto;
  padding: 0 16px;

  @media (min-width: 640px) {
    margin: 60px auto;
    padding: 0 24px;
  }

  @media (min-width: 1024px) {
    margin: 80px auto;
    padding: 0 32px;
  }
`

export const Header = styled.header`
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 24px;
`

export const Title = styled.h1`
  margin: 0;
  color: #111;
  font-size: 32px;
  letter-spacing: -0.03em;
`

export const Description = styled.p`
  max-width: 720px;
  margin: 0;
  color: #666;
  font-size: 15px;
  line-height: 1.5;
`

export const RunButton = styled.button`
  align-self: flex-start;
  margin-bottom: 28px;
  padding: 10px 16px;
  border: 1px solid #111;
  border-radius: 6px;
  background: #111;
  color: #fff;
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: #333;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const ErrorMessage = styled.p`
  margin: 0;
  color: #a21b1b;
  font-size: 14px;
`

export const ResultsSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 12px;
`

export const ResultsHeader = styled.div`
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
`

export const ResultsTitle = styled.h2`
  margin: 0;
  color: #111;
  font-size: 20px;
`

export const ResultsCount = styled.span`
  color: #777;
  font-size: 13px;
`

export const Message = styled.p`
  margin: 8px 0;
  color: #666;
  font-size: 15px;
`

export const ResultList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
`

export const ResultItem = styled.li`
  border: 1px solid #e2e2e2;
  border-radius: 10px;
  background: #fff;
  overflow: hidden;
`

export const ResultDetails = styled.details`
  &[open] {
    border-color: #c9c9c9;
  }
`

export const ResultSummary = styled.summary`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  cursor: pointer;
  list-style: none;

  &::-webkit-details-marker {
    display: none;
  }

  &::after {
    content: '+';
    flex: 0 0 auto;
    color: #888;
    font-size: 20px;
    font-weight: 300;
  }

  ${ResultDetails}[open] &::after {
    content: '-';
  }

  &:hover {
    background: #fafafa;
  }

  &:focus-visible {
    outline: 2px solid #333;
    outline-offset: -2px;
  }
`

export const PersonaInfo = styled.div`
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
`

export const PersonaName = styled.span`
  overflow: hidden;
  color: #151515;
  font-size: 16px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
`

export const PersonaMeta = styled.span`
  overflow: hidden;
  color: #777;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
`

export const SummaryScore = styled.span`
  flex: 0 0 auto;
  padding: 5px 8px;
  border-radius: 999px;
  background: #f1f3ff;
  color: #33408f;
  font-size: 12px;
  font-weight: 650;
`

export const ResultBody = styled.div`
  border-top: 1px solid #e8e8e8;
`

export const Tabs = styled.div`
  display: flex;
  gap: 4px;
  padding: 12px 16px 0;
  border-bottom: 1px solid #e8e8e8;
`

export const Tab = styled.button<{ $active: boolean }>`
  padding: 8px 10px 10px;
  border: 0;
  border-bottom: 2px solid ${({ $active }) => ($active ? '#111' : 'transparent')};
  background: transparent;
  color: ${({ $active }) => ($active ? '#111' : '#777')};
  font-size: 13px;
  font-weight: ${({ $active }) => ($active ? 650 : 500)};
  cursor: pointer;

  &:hover {
    color: #111;
  }

  &:focus-visible {
    outline: 2px solid #333;
    outline-offset: -2px;
  }
`

export const Panel = styled.div`
  padding: 16px;
`

export const RawOutput = styled.pre`
  max-height: 560px;
  margin: 0;
  padding: 16px;
  overflow: auto;
  border: 1px solid #e2e2e2;
  border-radius: 8px;
  background: #f7f7f7;
  color: #111;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre;
`

export const ResponseMeta = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-bottom: 16px;
  color: #777;
  font-size: 12px;
`

export const AnswerList = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
`

export const AnswerCard = styled.article`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  background: #fcfcfc;
`

export const AnswerHeader = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
`

export const AnswerTitle = styled.h3`
  margin: 0;
  color: #202020;
  font-size: 14px;
  line-height: 1.35;
`

export const AnswerKey = styled.span`
  flex: 0 0 auto;
  color: #888;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
`

export const SelectedAnswer = styled.p`
  margin: 0;
  color: #33408f;
  font-size: 14px;
  font-weight: 650;
  line-height: 1.4;
`

export const AnswerNote = styled.p`
  margin: 0;
  color: #777;
  font-size: 12px;
`

export const Distribution = styled.div`
  display: flex;
  flex-direction: column;
  gap: 7px;
`

export const DistributionRow = styled.div`
  display: grid;
  grid-template-columns: minmax(90px, 1fr) minmax(80px, 2fr) 42px;
  align-items: center;
  gap: 8px;
  color: #555;
  font-size: 11px;
`

export const DistributionLabel = styled.span`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`

export const DistributionTrack = styled.span`
  height: 7px;
  overflow: hidden;
  border-radius: 999px;
  background: #e9e9e9;
`

export const DistributionBar = styled.span<{ $value: number }>`
  display: block;
  width: ${({ $value }) => `${Math.max(0, Math.min(100, $value))}%`};
  height: 100%;
  border-radius: inherit;
  background: #6874c5;
`

export const DistributionValue = styled.span`
  color: #777;
  font-variant-numeric: tabular-nums;
  text-align: right;
`

export const UnknownValue = styled.pre`
  margin: 0;
  overflow: auto;
  color: #555;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
`

export const EmptyState = styled.p`
  margin: 0;
  color: #777;
  font-size: 14px;
`
