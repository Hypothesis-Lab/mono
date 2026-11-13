import styled from 'styled-components'

export const Root = styled.div`
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 14px;
  background: #fff;
`

export const ProductHero = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 4px;
`

export const ProductHeading = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
`

export const ProductName = styled.h3`
  margin: 0;
  color: #202020;
  font-size: 21px;
`

export const StatusBadge = styled.span<{ $status: 'success' | 'warning' }>`
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 2px 9px;
  border-radius: 999px;
  background: ${({ $status }) => ($status === 'success' ? '#e9f7ed' : '#fff4d6')};
  color: ${({ $status }) => ($status === 'success' ? '#21733a' : '#8a5a00')};
  font-size: 12px;
  font-weight: 700;
`

export const Metrics = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
`

export const Metric = styled.div`
  min-width: 74px;
  padding: 8px 10px;
  border: 1px solid #e4e4e4;
  border-radius: 7px;
  background: #fafafa;
  text-align: center;
`

export const MetricValue = styled.strong`
  display: block;
  color: #222;
  font-size: 18px;
`

export const MetricLabel = styled.span`
  color: #777;
  font-size: 11px;
`

export const FactSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 8px;
`

export const SectionHeading = styled.h4`
  display: flex;
  align-items: center;
  gap: 7px;
  margin: 0;
  color: #333;
  font-size: 15px;
`

export const SectionCount = styled.span`
  color: #888;
  font-size: 12px;
  font-weight: 400;
`

export const FactList = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 8px;
`

export const FactCard = styled.article`
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid #e3e3e3;
  border-radius: 7px;
  background: #fcfcfc;
`

export const FactHeader = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
`

export const FactSubject = styled.strong`
  color: #333;
  font-size: 13px;
`

export const FactStatement = styled.p`
  margin: 0;
  color: #333;
  font-size: 14px;
  line-height: 1.45;
`

export const EvidenceDetails = styled.details`
  margin-top: 2px;
  padding-top: 7px;
  border-top: 1px solid #ededed;
`

export const EvidenceSummary = styled.summary`
  color: #6d6d6d;
  cursor: pointer;
  font-size: 12px;
`

export const EvidenceNote = styled.p`
  margin: 8px 0 0;
  color: #666;
  font-size: 12px;
`

export const TagList = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
`

export const Tag = styled.span`
  padding: 3px 7px;
  border-radius: 4px;
  background: #f0f0f0;
  color: #666;
  font-size: 11px;
`

export const EvidenceList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 7px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
`

export const EvidenceItem = styled.li`
  padding-left: 9px;
  border-left: 2px solid #d8d8d8;
`

export const EvidenceQuote = styled.p`
  margin: 0;
  color: #555;
  font-size: 12px;
  line-height: 1.4;
`

export const EvidenceSource = styled.span`
  display: block;
  margin-top: 3px;
  color: #999;
  font-size: 11px;
  overflow-wrap: anywhere;
`

export const NoticeGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 8px;
`

export const Notice = styled.section<{ $tone: 'warning' | 'neutral' }>`
  padding: 11px 12px;
  border: 1px solid ${({ $tone }) => ($tone === 'warning' ? '#ead8a4' : '#ddd')};
  border-radius: 7px;
  background: ${({ $tone }) => ($tone === 'warning' ? '#fffaf0' : '#f8f8f8')};
`

export const NoticeTitle = styled.h5`
  margin: 0 0 7px;
  color: #444;
  font-size: 13px;
`

export const BulletList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 5px;
  margin: 0;
  padding-left: 17px;
  color: #555;
  font-size: 12px;
  line-height: 1.4;
`

export const QualityPanel = styled.section`
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 4px;
`

export const QualityItems = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
`

export const QualityItem = styled.div`
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 120px;
  padding: 8px 10px;
  border: 1px solid #e4e4e4;
  border-radius: 6px;
`

export const QualityLabel = styled.span`
  color: #888;
  font-size: 11px;
`

export const QualityValue = styled.strong`
  color: #333;
  font-size: 13px;
`

export const QualitySummary = styled.p`
  margin: 0;
  color: #666;
  font-size: 12px;
  line-height: 1.45;
`
