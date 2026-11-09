import styled from 'styled-components'

export const Page = styled.main`
  display: flex;
  flex-direction: column;
  max-width: 100%;
  margin: 40px auto;
  padding: 0 16px;

  @media (min-width: 640px) {
    max-width: 560px;
    margin: 60px auto;
    padding: 0 24px;
  }

  @media (min-width: 1024px) {
    max-width: 640px;
    margin: 80px auto;
    padding: 0 32px;
  }

  @media (min-width: 1280px) {
    padding: 0 40px;
  }
`

export const Title = styled.h1`
  font-size: 28px;
  margin-bottom: 24px;
  color: #111;
`

export const Form = styled.form`
  display: flex;
  flex-direction: column;
  gap: 16px;
`

export const Field = styled.label`
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 14px;
  color: #333;
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

export const AudiencesSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fafafa;
  margin-top: 16px;
  order: 3;
`

export const AudienceList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
`

type AnalysisPipelineState = 'completed' | 'active' | 'waiting' | 'pending' | 'error'

const pipelineStateColor = (state: AnalysisPipelineState): string => {
  if (state === 'completed') return '#2a8a2a'
  if (state === 'active') return '#33408f'
  if (state === 'waiting') return '#8a5a00'
  if (state === 'error') return '#b42318'
  return '#888'
}

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

export const AnalysisStatusTitle = styled.h3`
  margin: 4px 0 0;
  font-size: 18px;
  color: #111;
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
  font-size: 16px;
  color: #222;
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

export const AudienceInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
`

export const AudienceDescription = styled.span`
  font-size: 14px;
  font-weight: 600;
  color: #333;
  word-break: break-all;
`

export const AudienceMeta = styled.span`
  font-size: 12px;
  color: #888;
`

export const FilesSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fafafa;
  margin-top: 16px;
  order: 2;
`

export const SectionTitle = styled.h2`
  font-size: 18px;
  margin: 0;
  color: #111;
`

export const ResultsTitle = styled.h3`
  margin: 4px 0 0;
  font-size: 18px;
  color: #111;
`

export const FileList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
`

export const FileItem = styled.li`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #e2e2e2;
  border-radius: 6px;
  background: #fff;
`

export const FileInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
`

export const FileName = styled.span`
  font-size: 14px;
  font-weight: 600;
  color: #333;
  word-break: break-all;
`

export const FileMeta = styled.span`
  font-size: 12px;
  color: #888;
`

export const TypeBadge = styled.span<{ $type: 'product' | 'brandbook' | 'reference' | 'other' }>`
  padding: 2px 8px;
  border-radius: 10px;
  background: ${({ $type }) => ($type === 'brandbook' ? '#e3f2e8' : '#e8ecff')};
  color: ${({ $type }) => ($type === 'brandbook' ? '#1d6b3c' : '#33408f')};
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
`

export const UploadForm = styled.form`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid #e2e2e2;
`

export const FileInput = styled.input`
  font-size: 14px;
`

export const Select = styled.select`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
  background: #fff;
`

export const ButtonGroup = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
`

export const Button = styled.button<{ $primary?: boolean; $danger?: boolean; $success?: boolean }>`
  padding: 10px 16px;
  border: 1px solid ${({ $danger, $success }) => ($danger ? '#c00' : $success ? '#2a8a2a' : '#ccc')};
  border-radius: 6px;
  background: ${({ $primary, $danger, $success }) =>
    $primary ? '#111' : $danger ? '#c00' : $success ? '#2a8a2a' : '#fff'};
  color: ${({ $primary, $danger, $success }) => ($primary || $danger || $success ? '#fff' : '#111')};
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: ${({ $primary, $danger, $success }) =>
      $primary ? '#333' : $danger ? '#a00' : $success ? '#237023' : '#f5f5f5'};
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const Message = styled.p`
  font-size: 16px;
  color: #666;
`

export const ErrorMessage = styled.p`
  font-size: 16px;
  color: #c00;
`
